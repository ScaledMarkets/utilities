/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	"fmt"
	//"errors"
	"net"
	"strings"
	//"runtime/debug"	
	
	// SafeHarbor packages:
)

/*******************************************************************************
 * Determine the IP address of the host on which this function is executed.
 * Note that this address is local to the network in which the host exists -
 * it is probably not a public IP address.
 */
func DetermineIPAddress(adapter string) (string, error) {
	
	var ipaddr string
	var intfs []net.Interface
	var err error
	intfs, err = net.Interfaces()
	if err != nil { return "", err }
	for _, intf := range intfs {
		fmt.Println("Examining interface " + intf.Name)
		if intf.Name == adapter {
			var addrs []net.Addr
			addrs, err = intf.Addrs()
			if err != nil { return "", err }
			for _, addr := range addrs {
				fmt.Println("\tExamining address " + addr.String())
				ipaddr = strings.Split(addr.String(), "/")[0]
				var ip net.IP = net.ParseIP(ipaddr)
				if ip.To4() == nil {
					fmt.Println("\t\tskipping")
					continue // skip IP6 addresses
				}
				fmt.Println("Found " + addr.String() + " on network " + addr.Network());
				break
			}
			break
		}
	}
	return ipaddr, nil
}
