import { HttpAPI } from '../common/httpAPI'
import { Injectable } from '@angular/core';
import { Observable } from 'rxjs/Observable';

declare var _:any;

@Injectable()
export class RuntimeService {

    constructor(public httpAPI: HttpAPI) {
    }


    getRuntime(filter_s: string) {
        // return an observable
        return this.httpAPI.get('/runtime/info')
        .map( (responseData) => {
            return responseData.json();
        })
        .map((runtime_devs) => {
            let result = [];
            if (runtime_devs) {
                _.forEach(runtime_devs,function(value,key){
                  console.log("KEY: ",key);
                  console.log("FOREACH LOOP",value,key);
                    if(filter_s && filter_s.length > 0 ) {
                        console.log("maching: "+key+ "| filter: "+filter_s);
                        var re = new RegExp(filter_s, 'gi');
                        if (key.match(filter_s)){
                            result.push({'ID': key, 'value' :value});
                        }
                        console.log(key.match(re));
                    } else {
                            result.push({'ID': key, 'value' :value});
                    }
                });
            }
            console.log("result:",result);
            return result;
        });
    }

    getRuntimeById(id : string) {
        // return an observable
        return this.httpAPI.get('/runtime/info/'+id)
        .map( (responseData) =>
            responseData.json()
    )};

    changeDeviceActive(id : string, event : boolean) {
        // return an observable
        if (event) {
            return this.httpAPI.put('/runtime/activatedev/'+id,id)
            .map( (responseData) =>
                responseData.json()
            )
        } else {
            return this.httpAPI.put('/runtime/deactivatedev/'+id,id)
            .map( (responseData) =>
                responseData.json()
            )
        }
    };

    changeStateDebug(id : string, event : boolean) {
        // return an observable
        if (event) {
            return this.httpAPI.put('/runtime/actsnmpdbg/'+id,id)
            .map( (responseData) =>
                responseData.json()
            )
        } else {
            return this.httpAPI.put('/runtime/deactsnmpdbg/'+id,id)
            .map( (responseData) =>
                responseData.json()
            )
        }
    };

    changeLogLevel(id : string, level: string) {
        console.log(level);
        // return an observable
            return this.httpAPI.put('/runtime/setloglevel/'+id+'/'+level,[id,level])
            .map( (responseData) =>
                responseData.json()
            )
    };

    downloadLogFile(id : string) {
        // return an observable
        return this.httpAPI.get('/runtime/getdevicelog/'+id)
        .map( (res) => {
            console.log("service_response",res)
            //return new Blob([res.arrayBuffer()],{type: "application/octet-stream" })
            return new Blob([res['_body']],{type: "application/octet-stream" })
        })
    };

    forceFltUpdate(id : string) {
        // return an observable
        return this.httpAPI.get('/runtime/forcefltupdate/'+id)
        .map( (responseData) =>
            responseData.json()
        )
    };
}
