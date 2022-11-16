BINARY_NAME=PrinterEmulator
## build: builds all binaries
build-Windows:
	@env GOOS=windows GOARCH=amd64 go build -o ${BINARY_NAME}-Windows.exe
	@echo windows exe built!
build-Mac:
	@env GOOS=darwin GOARCH=amd64 go build -o ${BINARY_NAME}-OSX.exe
	@echo mac exe built!
build-Linux:
	@env GOOS=linux GOARCH=amd64 go build -o ${BINARY_NAME}-Linux.exe
	@echo linux exe built!
build: build-Windows build-Mac build-Linux