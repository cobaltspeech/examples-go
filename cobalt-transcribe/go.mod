module github.com/cobaltspeech/examples-go/cobalt-transcribe

go 1.19

replace github.com/cobaltspeech/cubicsvr/v5 => ../../cubicsvr

require (
	github.com/cobaltspeech/cubicsvr/tetracubic/v5 v5.0.0-20230224051945-9abd0eb47fc4
	github.com/cobaltspeech/cubicsvr/v5 v5.0.0-00010101000000-000000000000
	github.com/cobaltspeech/go-genproto v0.0.0-20230228070040-8b182b61673b
	github.com/cobaltspeech/log v0.1.12
	github.com/pelletier/go-toml/v2 v2.0.6
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	google.golang.org/grpc v1.53.0
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20230227214838-9b19f0bdc514 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)
