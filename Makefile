build:
	go build -trimpath -o goverter cmd/goverter/main.go

install: build
	cp goverter ~/go/bin
