module github.com/costinm/ugate/ext/gvisor

go 1.16

// go get -d -u   github.com/costinm/gvisor@tungate
// latest
replace gvisor.dev/gvisor => github.com/costinm/gvisor v0.0.0-20210509154143-a94fe58cda62

//replace gvisor.dev/gvisor => ../../../gvisor

replace github.com/costinm/ugate => ../..

require (
	github.com/costinm/ugate v0.0.0-20210221155556-10edd21fadbf
	gvisor.dev/gvisor v0.0.0-20210507193914-e691004e0c6c

)
