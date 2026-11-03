// Command nexa is the full ERP server entrypoint for the Nexa platform
// (core engine + all nexa_addons modules). Library code lives under core/.
package main

import (
	_ "nexa_addons/addonimports"

	"nexa/core/server"
)

func main() {
	server.Run()
}
