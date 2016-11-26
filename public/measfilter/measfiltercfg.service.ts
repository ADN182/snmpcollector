import { Http,Headers } from '@angular/http';
import { Injectable } from '@angular/core';
import * as _ from 'lodash';

@Injectable()
export class MeasFilterService {

    constructor(public http: Http) {
        console.log('Task Service created.', http);
    }

    addMeasFilter(dev) {
        var headers = new Headers();
        headers.append("Content-Type", 'application/json');
        return this.http.post('/measfilters',JSON.stringify(dev,function (key,value) {
                if ( key == 'EnableAlias' ) return ( value === "true" || value === true);
                if ( key == 'IDMeasurementCfg') {
                    if ( value == "" ) return null
                }
                return value;
        }), { headers: headers })
        .map( (responseData) => responseData.json());
    }

    editMeasFilter(dev, id) {
        var headers = new Headers();
        headers.append("Content-Type", 'application/json');
        console.log("DEV: ",dev);
        return this.http.put('/measfilters/'+id,JSON.stringify(dev,function (key,value) {
            if ( key == 'EnableAlias' ) return ( value === "true" || value === true);
            if ( key == 'IDMeasurementCfg') {
                if ( value == "" ) return null
            }
            return value;

        }), {  headers: headers   })
        .map( (responseData) => responseData.json());
    }

    getMeasFilter(filter_s: string) {
        // return an observable
        return this.http.get('/measfilters')
        .map( (responseData) => {
            return responseData.json();
        })
        .map((measfilter) => {
            console.log("MAP SERVICE",measfilter);
            let result = [];
            if (measfilter) {
                _.forEach(measfilter,function(value,key){
                    console.log("FOREACH LOOP",value,value.ID);
                    if(filter_s && filter_s.length > 0 ) {
                        console.log("maching: "+value.ID+ "filter: "+filter_s);
                        var re = new RegExp(filter_s, 'gi');
                        if (value.ID.match(re)){
                            result.push(value);
                        }
                        console.log(value.ID.match(re));
                    } else {
                        result.push(value);
                    }
                });
            }
            return result;
        });
    }
    getMeasFilterById(id : string) {
        // return an observable
        console.log("ID: ",id);
        return this.http.get('/measfilters/'+id)
        .map( (responseData) =>
            responseData.json()
    )};

    checkOnDeleteMeasFilter(id : string){
      return this.http.get('/measfilters/checkondel/'+id)
      .map( (responseData) =>
       responseData.json()
      ).map((deleteobject) => {
          console.log("MAP SERVICE",deleteobject);
          let result : any = {'ID' : id};
          _.forEach(deleteobject,function(value,key){
              result[value.Type] = [];
          });
          _.forEach(deleteobject,function(value,key){
              result[value.Type].Description=value.Action;
              result[value.Type].push(value.ObID);
          });
          return result;
      });
    };

    deleteMeasFilter(id : string) {
        // return an observable
        console.log("ID: ",id);
        console.log("DELETING");
        return this.http.delete('/measfilters/'+id)
        .map( (responseData) =>
         responseData.json()
        );
    };
}
