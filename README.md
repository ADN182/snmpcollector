# snmpcollector

## Run from master
If you want to build a package yourself, or contribute. Here is a guide for how to do that.

### Dependencies

- Go 1.5
- NodeJS

### Get Code

```
go get github.com/toni-moreno/snmpcollector
```

### Building the backend
Replace X.Y.Z by actual version number.
```
cd $GOPATH/src/github.com/toni-moreno/snmpcollector
go run build.go setup            (only needed once to install godep)
godep restore                    (will pull down all golang lib dependencies in your current GOPATH)
go run build.go build
```

### Building frontend assets

To build less to css for the frontend you will need a recent version of of node (v0.12.0),
npm (v2.5.0) and grunt (v0.4.5). Run the following:

```
npm install
./node_modules/.bin/typings install
ln -s ../node_modules/  public/node_modules
npm run tsc
```

### Recompile backend on source change
To rebuild on source change (requires that you executed godep restore)
```
go get github.com/Unknwon/bra
bra run

