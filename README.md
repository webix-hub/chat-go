Backed for Webix Chat
==================

### How to start

#### Run with docker

```bash
# build binary
CGO_ENABLED=0 GOARCH=386 go build .
# run docker
docker-compose up -d
```

the command will start backend at http://localhost:3000

you can change the host name and port in docker-compose.yml

#### Standalone run

- create config.yml with DB access config

```
server:
  port: ":8040"
  public: "http://localhost:8040"
db:
  path: "./db.sqlite"
```

above config is for sqlite DB, if you want to use mysql change it like 

```yaml
db:
  host: localhost
  port: 3306
  user: root
  password: 1
  database: files
```

you need to create the database ( code will init all necessary tables on its own )

- start the backend

```bash
go build
./chat
```

### group calls

To organize group calls, service uses [livekit library](https://livekit.io/). So, to have this feature you need to deploy the instance of livekit on your infrastructure. It can be done through docker ( check the docker-compose.yml ) or as a standalone software ( check instructions at https://livekit.io )


#### Other ways of configuration

Configuration can be done through config.yml file or through env vars
For example of env usage, check docker-compose.yml

