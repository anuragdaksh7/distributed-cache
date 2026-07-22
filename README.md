go run ./cmd/router

go run ./cmd/node --port=7001
go run ./cmd/node --port=7002
go run ./cmd/node --port=7003


### add resp parser
ncat localhost 7001