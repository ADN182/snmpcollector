package main

import (
	"flag"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// GeneralConfig has miscelaneous configuration options
type GeneralConfig struct {
	InstanceID string `toml:"instanceID"`
	LogDir     string `toml:"logdir"`
	HomeDir    string `toml:"homedir"`
	DataDir    string `toml:"datadir"`
	LogLevel   string `toml:"loglevel"`
}

var (
	version    string
	commit     string
	branch     string
	buildstamp string
)

var (
	log        = logrus.New()
	quit       = make(chan struct{})
	startTime  = time.Now()
	showConfig bool
	getversion bool
	httpPort   = 8080
	appdir     = os.Getenv("PWD")
	homeDir    string
	pidFile    string
	logDir     = filepath.Join(appdir, "log")
	confDir    = filepath.Join(appdir, "conf")
	dataDir    = confDir
	configFile = filepath.Join(confDir, "config.toml")

	cfg = struct {
		General      GeneralConfig
		Database     DatabaseCfg
		Selfmon      SelfMonConfig
		Metrics      map[string]*SnmpMetricCfg
		Measurements map[string]*InfluxMeasurementCfg
		MFilters     map[string]*MeasFilterCfg
		GetGroups    map[string]*MGroupsCfg
		SnmpDevice   map[string]*SnmpDeviceCfg
		Influxdb     map[string]*InfluxCfg
		HTTP         HTTPConfig
	}{}
	//mutex for devices map
	mutex sync.Mutex
	//runtime devices
	devices map[string]*SnmpDevice
	//runtime output db's
	influxdb map[string]*InfluxDB
	// for synchronize  deivce specific goroutines
	GatherWg sync.WaitGroup
	SenderWg sync.WaitGroup
)

func writePIDFile() {
	if pidFile == "" {
		return
	}

	// Ensure the required directory structure exists.
	err := os.MkdirAll(filepath.Dir(pidFile), 0700)
	if err != nil {
		log.Fatal(3, "Failed to verify pid directory", err)
	}

	// Retrieve the PID and write it.
	pid := strconv.Itoa(os.Getpid())
	if err := ioutil.WriteFile(pidFile, []byte(pid), 0644); err != nil {
		log.Fatal(3, "Failed to write pidfile", err)
	}
}

func flags() *flag.FlagSet {
	var f flag.FlagSet
	f.BoolVar(&getversion, "version", getversion, "display de version")
	f.BoolVar(&showConfig, "showconf", showConfig, "show all devices config and exit")
	f.StringVar(&configFile, "config", configFile, "config file")
	f.IntVar(&httpPort, "http", httpPort, "http port")
	f.StringVar(&logDir, "logs", logDir, "log directory")
	f.StringVar(&homeDir, "home", homeDir, "home directory")
	f.StringVar(&dataDir, "data", dataDir, "Data directory")
	f.StringVar(&pidFile, "pidfile", pidFile, "path to pid file")
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		f.VisitAll(func(flag *flag.Flag) {
			format := "%10s: %s\n"
			fmt.Fprintf(os.Stderr, format, "-"+flag.Name, flag.Usage)
		})
		fmt.Fprintf(os.Stderr, "\nAll settings can be set in config file: %s\n", configFile)
		os.Exit(1)

	}
	return &f
}

/*
initMetricsCfg this function does 2 things
1.- Initialice id from key of maps for all SnmpMetricCfg and InfluxMeasurementCfg objects
2.- Initialice references between InfluxMeasurementCfg and SnmpMetricGfg objects
*/

func initMetricsCfg() error {
	//TODO:
	// - check duplicates OID's => warning messages
	//Initialize references to SnmpMetricGfg into InfluxMeasurementCfg
	log.Debug("--------------------Initializing Config metrics-------------------")
	log.Debug("Initializing SNMPMetricconfig...")
	for mKey, mVal := range cfg.Metrics {
		err := mVal.Init()
		if err != nil {
			log.Warnln("Error in Metric config:", err)
			//if some error int the format the metric is deleted from the config
			delete(cfg.Metrics, mKey)
		}
	}
	log.Debug("Initializing MEASSUREMENTSconfig...")
	for mKey, mVal := range cfg.Measurements {
		err := mVal.Init(&cfg.Metrics)
		if err != nil {
			log.Warnln("Error in Measurement config:", err)
			//if some error int the format the metric is deleted from the config
			delete(cfg.Metrics, mKey)
		}

		log.Debugf("FIELDMETRICS: %+v", mVal.fieldMetric)
	}
	log.Debug("-----------------------END Config metrics----------------------")
	return nil
}

//PrepareInfluxDBs review all configured db's in the SQL database
// and check if exist at least a "default", if not creates a dummy db which does nothing
func PrepareInfluxDBs() map[string]*InfluxDB {
	idb := make(map[string]*InfluxDB)

	var defFound bool
	for k, c := range cfg.Influxdb {
		//Inticialize each SNMP device
		if k == "default" {
			defFound = true
		}
		dev := InfluxDB{
			cfg:     c,
			dummy:   false,
			started: false,
			Sent:    0,
			Errors:  0,
		}
		idb[k] = &dev
	}
	if defFound == false {
		//no devices configured  as default device we need to set some device as itcan send data transparant to snmpdevices goroutines
		log.Warn("No Output default found influxdb devices found !!")
		idb["default"] = influxdbDummy
	}
	return idb
}

func initSelfMonitoring(idb map[string]*InfluxDB) {
	log.Debugf("INFLUXDB2: %+v", idb)
	if cfg.Selfmon.Enabled && !showConfig {
		if val, ok := idb["default"]; ok {
			//only executed if a "default" influxdb exist
			val.Init()
			val.StartSender(&SenderWg)

			cfg.Selfmon.Init()
			cfg.Selfmon.setOutput(val)

			log.Printf("SELFMON enabled %+v", cfg.Selfmon)
			//Begin the statistic reporting
			cfg.Selfmon.StartGather(&GatherWg)
		} else {
			cfg.Selfmon.Enabled = false
			log.Errorf("SELFMON disabled becaouse of no default db found !!! SELFMON[ %+v ]  INFLUXLIST[ %+v]\n", cfg.Selfmon, idb)
		}
	} else {
		log.Printf("SELFMON disabled %+v\n", cfg.Selfmon)
	}
}

func init() {
	//Log format
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.Formatter = customFormatter
	customFormatter.FullTimestamp = true
	//----

	// parse first time to see if config file is being specified
	f := flags()
	f.Parse(os.Args[1:])

	if getversion {
		t, _ := strconv.ParseInt(buildstamp, 10, 64)
		fmt.Printf("snmpcollector v%s (git: %s ) built at [%s]\n", version, commit, time.Unix(t, 0).Format("2006-01-02 15:04:05"))
		os.Exit(0)
	}

	// now load up config settings
	if _, err := os.Stat(configFile); err == nil {
		viper.SetConfigFile(configFile)
		confDir = filepath.Dir(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("/etc/snmpcollector/")
		viper.AddConfigPath("/opt/snmpcollector/conf/")
		viper.AddConfigPath("./conf/")
		viper.AddConfigPath(".")
	}
	err := viper.ReadInConfig()
	if err != nil {
		log.Errorf("Fatal error config file: %s \n", err)
		os.Exit(1)
	}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Errorf("Fatal error config file: %s \n", err)
		os.Exit(1)
	}

	if len(cfg.General.LogDir) > 0 {
		logDir = cfg.General.LogDir
		os.Mkdir(logDir, 0755)
	}
	if len(cfg.General.LogLevel) > 0 {
		l, _ := logrus.ParseLevel(cfg.General.LogLevel)
		log.Level = l
	}
	if len(cfg.General.DataDir) > 0 {
		dataDir = cfg.General.DataDir
	}
	if len(cfg.General.HomeDir) > 0 {
		homeDir = cfg.General.HomeDir
	}
	//check if exist public dir in home
	if _, err := os.Stat(filepath.Join(homeDir, "public")); err != nil {
		log.Warnf("There is no public (www) directory on [%s] directory", homeDir)
		if len(homeDir) == 0 {
			homeDir = appdir
		}
	}
	//
	log.Infof("Set Default directories : \n   - Exec: %s\n   - Config: %s\n   -Logs: %s\n -Home: %s\n", appdir, confDir, logDir, homeDir)
}

//GetDevice is a safe method to get a Device Object
func GetDevice(id string) (*SnmpDevice, error) {
	var dev *SnmpDevice
	var ok bool
	mutex.Lock()
	if dev, ok = devices[id]; !ok {
		return nil, fmt.Errorf("there is not any device with id %s running", id)
	}
	mutex.Unlock()
	return dev, nil
}

func GetDevStats() map[string]*devStat {
	devstats := make(map[string]*devStat)
	mutex.Lock()
	for k, v := range devices {
		devstats[k] = v.GetBasicStats()
	}
	mutex.Unlock()
	return devstats
}

func StopInfluxOut(idb map[string]*InfluxDB) {
	for k, v := range idb {
		log.Infof("Stopping Influxdb out %s", k)
		v.StopSender()
	}
}

func ReleaseInfluxOut(idb map[string]*InfluxDB) {
	for k, v := range idb {
		log.Infof("Release Influxdb resources %s", k)
		v.End()
	}
}

// ProcessStop stop all device goroutines
func DeviceProcessStop() {
	mutex.Lock()
	for _, c := range devices {
		c.StopGather()
	}
	mutex.Unlock()
}

// ProcessStart start all devices goroutines
func DeviceProcessStart() {
	mutex.Lock()
	for _, c := range devices {
		c.StartGather(&GatherWg)
	}
	mutex.Unlock()
}

func ReleaseDevices() {
	mutex.Lock()
	for _, c := range devices {
		c.End()
	}
	mutex.Unlock()
}

// LoadConf call to initialize alln configurations
func LoadConf() {
	//Load all database info to Cfg struct
	cfg.Database.LoadConfig()
	//Prepare the InfluxDataBases Configuration
	influxdb = PrepareInfluxDBs()

	// beginning self monitoring process if needed.( before each other gorotines could begin)

	initSelfMonitoring(influxdb)

	//Initialize Device Metrics CFG

	initMetricsCfg()

	//Initialize Device Runtime map

	devices = make(map[string]*SnmpDevice)

	for k, c := range cfg.SnmpDevice {
		//Inticialize each SNMP device and put pointer to the global map devices
		dev := NewSnmpDevice(c)
		dev.SetSelfMonitoring(&cfg.Selfmon)
		//send db's map to initialize each one its own db if needed and not yet initialized
		if !showConfig {
			outdb, _ := dev.GetOutSenderFromMap(influxdb)
			outdb.Init()
			outdb.StartSender(&SenderWg)
		}
		mutex.Lock()
		devices[k] = dev
		mutex.Unlock()
	}

	// only run when one needs to see the interface names of the device
	if showConfig {
		mutex.Lock()
		for _, c := range devices {
			fmt.Println("\nSNMP host:", c.cfg.ID)
			fmt.Println("=========================================")
			c.printConfig()
		}
		mutex.Unlock()
		os.Exit(0)
	}
	//beginning  the gather process
}

// ReloadConf call to reinitialize alln configurations
func ReloadConf() time.Duration {
	start := time.Now()
	log.Info("RELOADCONF: begin device Gather processes stop...")
	//stop all device prcesses
	DeviceProcessStop()
	log.Info("RELOADCONF: begin selfmon Gather processes stop...")
	//stop the selfmon process
	cfg.Selfmon.StopGather()
	log.Info("RELOADCONF: waiting for all Gather gorotines stop...")
	//wait until Done
	GatherWg.Wait()
	log.Info("RELOADCONF: releasing Device Resources")
	ReleaseDevices()
	log.Info("RELOADCONF: releasing Seflmonitoring Resources")
	cfg.Selfmon.End()
	log.Info("RELOADCONF: begin sender processes stop...")
	//stop all Output Emmiter
	//log.Info("DEBUG Gather WAIT %+v", GatherWg)
	//log.Info("DEBUG SENDER WAIT %+v", SenderWg)
	StopInfluxOut(influxdb)
	log.Info("RELOADCONF: waiting for all Sender gorotines stop..")
	SenderWg.Wait()
	log.Info("RELOADCONF: releasing Sender Resources")
	ReleaseInfluxOut(influxdb)

	log.Info("RELOADCONF: ĺoading configuration Again...")
	LoadConf()
	log.Info("RELOADCONF: Starting all device processes again...")
	DeviceProcessStart()
	return time.Since(start)
}

func main() {

	defer func() {
		//errorLog.Close()
	}()
	writePIDFile()
	//Init BD config

	InitDB(&cfg.Database)

	LoadConf()

	DeviceProcessStart()

	var port int
	if cfg.HTTP.Port > 0 {
		port = cfg.HTTP.Port
	} else {
		port = httpPort
	}

	if port > 0 {
		webServer(filepath.Join(homeDir, "public"), port)
	} else {
		<-quit
	}
}
