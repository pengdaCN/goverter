build:
	go build -trimpath -o goverter cmd/goverter/main.go

install:
	cd cmd/goverter && go install
