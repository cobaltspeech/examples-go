module github.com/cobaltspeech/examples-go/cobalt-transcribe

go 1.19

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/cobaltspeech/examples-go/pkg v0.0.0-20220927145534-5a04be129ac9
	github.com/cobaltspeech/log v0.1.12
	github.com/cobaltspeech/sdk-cubic/grpc/go-cubic v1.6.0
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	golang.org/x/net v0.0.0-20210913180222-943fd674d43e // indirect
	golang.org/x/sys v0.0.0-20210915083310-ed5796bab164 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20210909211513-a8c4777a87af // indirect
	google.golang.org/grpc v1.40.0 // indirect
)

replace github.com/cobaltspeech/examples-go/pkg v0.0.0-20220927145534-5a04be129ac9 => ../pkg
