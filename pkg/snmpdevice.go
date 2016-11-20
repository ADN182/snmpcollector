package main

import (
	"fmt"
	"io/ioutil"
	olog "log"
	"os"

	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/soniah/gosnmp"
)

// SnmpDevice contains all runtime device related device configu ns and state
type SnmpDevice struct {
	cfg *SnmpDeviceCfg
	log *logrus.Logger
	//basic sistem info
	SysInfo *SysInfo
	//runtime built TagMap
	TagMap map[string]string
	//Measurements array

	Measurements []*InfluxMeasurement

	//SNMP and Influx Clients config
	snmpClient *gosnmp.GoSNMP
	Influx     *InfluxDB
	LastError  time.Time
	//Runtime stats
	Requests int64
	Gets     int64
	Errors   int64
	//runtime controls

	ReloadLoopsPending int

	DeviceActive    bool
	DeviceConnected bool
	StateDebug      bool

	chDebug    chan bool
	chEnabled  chan bool
	chLogLevel chan string
}

//RTActivate change activatio state in runtime
func (d *SnmpDevice) RTActivate(activate bool) {
	d.chEnabled <- activate
}

//RTActSnmpDebug change snmp debug runtime
func (d *SnmpDevice) RTActSnmpDebug(activate bool) {
	d.chDebug <- activate
}

//RTSetLogLevel
func (d *SnmpDevice) RTSetLogLevel(level string) {
	d.chLogLevel <- level
}

/*
InitDevSnmpInfo  does the following
- look for all defined measurments from the template grups
- allocate and fill array for all measurements defined for this device
- for each indexed measurement  load device labels from IndexedOID and fiter them if defined measurement filters.
- Initialice each SnmpMetric from each measuremet.
*/
//InitDevSnmpInfo generte all internal structs from SNMP device
func (d *SnmpDevice) InitDevSnmpInfo() {

	//Alloc array
	d.Measurements = make([]*InfluxMeasurement, 0, 0)
	d.log.Debugf("-----------------Init device measurements from groups %s------------------", d.cfg.Host)
	//for this device get MeasurementGroups and search all measurements

	for _, devMeas := range d.cfg.MeasurementGroups {
		//Selecting all Metric Groups that matches with device.MeasurementGroups
		selGroups := make(map[string]*MGroupsCfg, 0)
		var RegExp = regexp.MustCompile(devMeas)
		for key, val := range cfg.GetGroups {
			if RegExp.MatchString(key) {
				selGroups[key] = val
			}
		}
		d.log.Debugf("SNMP device %s has this SELECTED GROUPS: %+v", d.cfg.ID, selGroups)
		//Only For selected Groups we will get all selected measurements and we will remove repeated values
		var selMeas []string
		for key, val := range selGroups {
			d.log.Debugln("Selecting from group", key)
			for _, item := range val.Measurements {
				d.log.Debugln("Selecting measurements", item, "from group", key)
				selMeas = append(selMeas, item)
			}
		}
		//remove duplicated measurements if needed
		selMeasUniq := removeDuplicatesUnordered(selMeas)
		//Now we know what measurements names  will send influx from this device

		d.log.Debugln("DEVICE MEASUREMENT: ", devMeas, "HOST: ", d.cfg.Host)
		for _, val := range selMeasUniq {
			//check if measurement exist
			if mVal, ok := cfg.Measurements[val]; !ok {
				d.log.Warnln("no measurement configured with name ", val, "in host :", d.cfg.Host)
			} else {
				d.log.Debugln("MEASUREMENT CFG KEY:", val, " VALUE ", mVal.Name)

				//creating a new measurement runtime object and asigning to array
				imeas, err := NewInfluxMeasurement(mVal, d.log, d.snmpClient)
				if err != nil {
					d.log.Errorf("Error on measurement initialization on host %s: Error: %s", d.cfg.ID, err)
					continue
				}
				d.Measurements = append(d.Measurements, imeas)
			}
		}
	}

	/*For each  measurement look for filters and  Add to the measurment with this Filter after it initializes the runtime for the measurment  	*/

	for _, m := range d.Measurements {
		//check for filters asociated with this measurement
		var mfilter *MeasFilterCfg
		for _, f := range d.cfg.MeasFilters {
			//we seach if exist in the filter Database
			if filter, ok := cfg.MFilters[f]; ok {
				if filter.IDMeasurementCfg == m.ID {
					mfilter = filter
					break
				}
			}
		}
		if mfilter != nil {
			d.log.Debugf("filters %s found for device %s and measurment %s ", mfilter.ID, d.cfg.ID, m.cfg.ID)
			err := m.AddFilter(mfilter)
			if err != nil {
				d.log.Errorf("Error on initialize Filter for Measurement %s , Error:%s no data will be gathered for this measurement", m.cfg.ID, err)
			}
		} else {
			d.log.Debugf("no filters found for device %s and measurment %s", d.cfg.ID, m.cfg.ID)
		}
		//Initialize internal structs after
		m.InitBuildRuntime()
		//Get Data First Time ( usefull for counters)
		m.GetData()
	}
	//Initialize all snmpMetrics  objects and OID array
	//get data first time
	// useful to inicialize counter all value and test device snmp availability

}

/*
Init  does the following

- Initialize not set variables to some defaults
- Initialize logfile for this device
- Initialize comunication channels and initial device state
*/
func (d *SnmpDevice) Init(c *SnmpDeviceCfg) error {
	if c == nil {
		return fmt.Errorf("Error on initialice device, configuration struct is nil")
	}
	d.cfg = c
	log.Infof("Initializing device %s\n", d.cfg.ID)

	//Init Logger

	if len(d.cfg.LogFile) == 0 {
		d.cfg.LogFile = cfg.General.LogDir + "/" + d.cfg.ID + ".log"

	}
	if len(d.cfg.LogLevel) == 0 {
		d.cfg.LogLevel = "info"
	}

	f, _ := os.OpenFile(d.cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	d.log = logrus.New()
	d.log.Out = f
	l, _ := logrus.ParseLevel(d.cfg.LogLevel)
	d.log.Level = l
	//Formatter for time
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	d.log.Formatter = customFormatter
	customFormatter.FullTimestamp = true

	d.ReloadLoopsPending = d.cfg.UpdateFltFreq

	//Init channels
	d.chDebug = make(chan bool)
	d.chEnabled = make(chan bool)
	d.chLogLevel = make(chan string)
	d.DeviceActive = d.cfg.Active

	//Init Device Tags

	d.TagMap = make(map[string]string)
	if len(d.cfg.DeviceTagName) == 0 {
		d.cfg.DeviceTagName = "device"
	}

	var val string

	switch d.cfg.DeviceTagValue {
	case "id":
		val = d.cfg.ID
	case "host":
		val = d.cfg.Host
	default:
		val = d.cfg.ID
		d.log.Warnf("Unkwnown DeviceTagValue %s set ID (%s) as value", d.cfg.DeviceTagValue, val)
	}

	d.TagMap[d.cfg.DeviceTagName] = val

	if len(d.cfg.ExtraTags) > 0 {
		for _, tag := range d.cfg.ExtraTags {
			s := strings.Split(tag, "=")
			if len(s) == 2 {
				key, value := s[0], s[1]
				d.TagMap[key] = value
			} else {
				d.log.Errorf("Error on tag definition TAG=VALUE [ %s ]", tag)
			}
		}
	} else {
		d.log.Warnf("No map detected in device %s\n", d.cfg.ID)
	}

	return nil
}

//InitSnmpConnect does the  SNMP client conection and retrieve system info

func (d *SnmpDevice) InitSnmpConnect() error {
	client, sysinfo, err := SnmpClient(d.cfg, d.log)
	if err != nil {
		d.DeviceConnected = false
		d.log.Errorf("Client connect error to device: %s  error :%s", d.cfg.ID, err)
		d.snmpClient = nil
		return err
	}

	d.log.Infof("SNMP connection stablished Successfully")
	d.snmpClient = client
	d.SysInfo = sysinfo
	d.DeviceConnected = true
	return nil
}

func (d *SnmpDevice) printConfig() {

	d.InitDevSnmpInfo()
	fmt.Printf("Host: %s Port: %d Version: %s\n", d.cfg.Host, d.cfg.Port, d.cfg.SnmpVersion)
	fmt.Printf("----------------------------------------------\n")
	for _, vM := range d.Measurements {
		fmt.Printf(" Measurement : %s\n", vM.cfg.ID)
		fmt.Printf(" ----------------------------------------------------------\n")
		vM.printConfig()
	}
}

func (d *SnmpDevice) incRequests() {
	atomic.AddInt64(&d.Requests, 1)
}

func (d *SnmpDevice) addRequests(n int64) {
	atomic.AddInt64(&d.Requests, n)
}
func (d *SnmpDevice) incGets() {
	atomic.AddInt64(&d.Gets, 1)
}
func (d *SnmpDevice) addGets(n int64) {
	atomic.AddInt64(&d.Gets, 1)
}

func (d *SnmpDevice) incErrors() {
	atomic.AddInt64(&d.Errors, 1)
}

func (d *SnmpDevice) addErrors(n int64) {
	atomic.AddInt64(&d.Errors, n)
}

/*/ DebugLog returns a logger handler for snmp debug data
func (d *SnmpDevice) DebugLog() *olog.Logger {
	name := filepath.Join(logDir, "snmpdebug_"+strings.Replace(d.cfg.ID, ".", "-", -1)+".log")
	if l, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644); err == nil {
		return olog.New(l, "", 0)
	} else {
		fmt.Fprintln(os.Stderr, err)
		return nil
	}
}*/

//Gather Main GoRutine method to begin snmp data collecting
func (d *SnmpDevice) Gather(wg *sync.WaitGroup) {

	if d.DeviceActive && d.DeviceConnected {
		d.log.Infof("Begin first InidevInfo")
		startSnmp := time.Now()
		d.InitDevSnmpInfo()
		elapsedSnmp := time.Since(startSnmp)
		d.log.Infof("snmpdevice [%s] snmp INIT runtime measurments/filters took [%s] ", d.cfg.ID, elapsedSnmp)
	} else {
		d.log.Infof("Can not initialize this device: Is Active: %t  |  Conection Active: %t ", d.DeviceActive, d.snmpClient != nil)
	}

	d.log.Infof("Beginning gather process for device %s (%s)", d.cfg.ID, d.cfg.Host)

	s := time.Tick(time.Duration(d.cfg.Freq) * time.Second)
	for {
		//if active
		if d.DeviceActive {

			//check if device is online
			if d.DeviceConnected == false {
				err := d.InitSnmpConnect()
				if err == nil {
					startSnmp := time.Now()
					d.InitDevSnmpInfo()
					elapsedSnmp := time.Since(startSnmp)
					d.log.Infof("snmpdevice [%s] snmp INIT runtime measurments/filters took [%s] ", d.cfg.ID, elapsedSnmp)
					//device not initialized
				}
			} else {
				//device actie and connected
				/*******************************************
				 *
				 * Reload Indexes/Filters process(if needed)
				 *
				 *******************************************/
				//Check if reload needed with d.ReloadLoopsPending if a posivive value on negative this will disabled
				if d.ReloadLoopsPending > 0 {
					d.ReloadLoopsPending--
				}
				if d.ReloadLoopsPending == 0 {
					startIdxUpdateStats := time.Now()
					for _, m := range d.Measurements {
						if m.cfg.GetMode == "value" {
							continue
						}
						changed, err := m.UpdateFilter()
						if err != nil {
							d.log.Errorf("Error on update Indexes/filter : ERR: %s", err)
							continue
						}
						if changed {
							m.InitBuildRuntime()
						}
					}
					d.ReloadLoopsPending = d.cfg.UpdateFltFreq
					elapsedIdxUpdateStats := time.Since(startIdxUpdateStats)
					d.log.Infof("snmpdevice [%s] Index reload took [%s]", d.cfg.ID, elapsedIdxUpdateStats)
				}
				/*************************
				 *
				 * SNMP Gather data process
				 *
				 ***************************/

				bpts := d.Influx.BP()
				startSnmpStats := time.Now()
				for _, m := range d.Measurements {
					d.log.Debugf("----------------Processing measurement : %s", m.cfg.ID)

					nGets, nErrors, _ := m.GetData()

					if nGets > 0 {
						d.addGets(nGets)
					}
					if nErrors > 0 {
						d.addErrors(nErrors)
					}
					//prepare batchpoint
					points := m.GetInfluxPoint(d.TagMap)
					(*bpts).AddPoints(points)
				}

				elapsedSnmpStats := time.Since(startSnmpStats)
				d.log.Infof("snmpdevice [%s] snmp pooling took [%s] ", d.cfg.ID, elapsedSnmpStats)
				/*************************
				 *
				 * Send data to InfluxDB process
				 *
				 ***************************/

				startInfluxStats := time.Now()
				d.Influx.Send(bpts)
				elapsedInfluxStats := time.Since(startInfluxStats)
				d.log.Infof("snmpdevice [%s] influx send took [%s]", d.cfg.ID, elapsedInfluxStats)

			}
		} else {
			d.log.Infof("snmpdevice [%s] Gather process is dissabled", d.cfg.ID)
		}
	LOOP:
		for {
			select {
			case <-s:
				break LOOP
			case debug := <-d.chDebug:
				d.StateDebug = debug
				d.log.Infof("DEBUG  ACTIVE %s [%t] ", d.cfg.ID, debug)
				if debug {
					d.log.Info("Activating snmp debug for this device")
					d.snmpClient.Logger = SnmpDebugLog(d.cfg.ID)
				} else {
					d.log.Info("De Activating snmp debug for this device")
					d.snmpClient.Logger = olog.New(ioutil.Discard, "", 0)
				}
			case status := <-d.chEnabled:
				d.DeviceActive = status
				d.log.Infof("STATUS  ACTIVE %s [%t] ", d.cfg.ID, status)
			case level := <-d.chLogLevel:
				l, err := logrus.ParseLevel(level)
				if err != nil {
					d.log.Warnf("ERROR on Changing LOGLEVEL in %s to [%t] ", d.cfg.ID, level)
				}
				d.log.Level = l
				d.log.Infof("CHANGED LOGLEVEL %s [%s] ", d.cfg.ID, level)
			}
		}
	}
	wg.Done()
}
