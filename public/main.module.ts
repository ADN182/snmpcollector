//Auth examples from: https://github.com/auth0-blog/angular2-authentication-sample
import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { RouterModule } from '@angular/router';
import { HttpModule } from '@angular/http';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';

import { AUTH_PROVIDERS } from 'angular2-jwt';

import { AuthGuard } from './common/auth.guard';
import { Home } from './home/home';
import { Login } from './login/login';
import { App } from './app/app';

import { routes } from './app/app.routes';
//common
import { ControlMessagesComponent } from './common/control-messages.component';
import { MultiselectDropdownModule } from './common/multiselect-dropdown'
//snmpcollector components

import { SnmpDeviceCfgComponent } from './home/snmpdevicecfg.component';
import { SnmpMetricCfgComponent } from './home/snmpmetriccfg.component';
import { InfluxMeasCfgComponent } from './home/influxmeascfg.component';
import { MeasGroupCfgComponent } from './home/measgroupcfg.component';
import { MeasFilterCfgComponent } from './home/measfiltercfg.component';
import { InfluxServerCfgComponent } from './home/influxservercfg.component';



import { AccordionModule } from 'ng2-bootstrap/ng2-bootstrap';
import { TooltipModule } from 'ng2-bootstrap/ng2-bootstrap';


@NgModule({
  bootstrap: [App],
  declarations: [
    ControlMessagesComponent,
    SnmpDeviceCfgComponent,
    SnmpMetricCfgComponent,
    InfluxMeasCfgComponent,
    MeasGroupCfgComponent,
    MeasFilterCfgComponent,
    InfluxServerCfgComponent,
    Home,
    Login,
    App,
  ],
  imports: [
    HttpModule, BrowserModule, FormsModule, ReactiveFormsModule,MultiselectDropdownModule, AccordionModule, TooltipModule,
    RouterModule.forRoot(routes, {
    //  useHash: true
    })
  ],
  providers: [
    AuthGuard, ...AUTH_PROVIDERS
  ]
})
export class AppModule {}
