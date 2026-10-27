package util

import "fmt"

func LocalAddr(port int, isNetworkPrintingEnabled bool) string {
	return fmt.Sprintf("%s:%d", GetLocalIP(isNetworkPrintingEnabled), port)
}

func GetPrinterUrl(port int, isNetworkPrintingEnabled bool, id string) string {
	return fmt.Sprintf("%s/p/%s", LocalAddr(port, isNetworkPrintingEnabled), id)
}
