package util

import (
	"net"
	"regexp"
	"strings"
	"testing"

	"obox-app/internal/testutil"
)

// cidrRe matches a valid CIDR notation, e.g. "192.168.1.0/24".
var cidrRe = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+/\d+$`)

var knownLinuxFirewalls = map[string]bool{
	"firewalld": true,
	"ufw":       true,
	"nftables":  true,
	"":          true,
}

func TestFormatSubnet(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		mask net.IPMask
		want string
	}{
		{
			name: "class C /24",
			ip:   net.ParseIP("192.168.1.42").To4(),
			mask: net.CIDRMask(24, 32),
			want: "192.168.1.0/24",
		},
		{
			name: "class B /16",
			ip:   net.ParseIP("10.20.30.40").To4(),
			mask: net.CIDRMask(16, 32),
			want: "10.20.0.0/16",
		},
		{
			name: "/32 host route",
			ip:   net.ParseIP("172.16.5.1").To4(),
			mask: net.CIDRMask(32, 32),
			want: "172.16.5.1/32",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSubnet(tc.ip, tc.mask)
			testutil.ExpectedEqual(t, got, tc.want)
		})
	}
}

func TestGetLocalIP_Disabled(t *testing.T) {
	testutil.ExpectedEqual(t, GetLocalIP(false), "127.0.0.1")
}

func TestGetLocalIP_Enabled(t *testing.T) {
	ip := GetLocalIP(true)
	testutil.ExpectedTrue(t, ip != "", "expected non-empty IP")
	parsed := net.ParseIP(ip)
	testutil.ExpectedTrue(t, parsed != nil, "expected a valid IP, got "+ip)
	testutil.ExpectedTrue(t, parsed.To4() != nil || ip == "127.0.0.1",
		"expected an IPv4 address, got "+ip)
}

func TestGetNetworkInfo(t *testing.T) {
	info := GetNetworkInfo()

	testutil.ExpectedTrue(t, info.IP != "", "IP must not be empty")
	parsed := net.ParseIP(info.IP)
	testutil.ExpectedTrue(t, parsed != nil, "IP must be valid, got: "+info.IP)

	if info.Subnet != "" {
		testutil.ExpectedTrue(t, cidrRe.MatchString(info.Subnet),
			"Subnet must be CIDR notation, got: "+info.Subnet)
	}

	testutil.ExpectedEqual(t, info.Interface != "", info.Subnet != "",
		"Interface and Subnet should be set together")
}

func TestRunCmd_Success(t *testing.T) {
	out, err := runCmd("echo", "hello")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedTrue(t, strings.Contains(out, "hello"),
		"expected output to contain 'hello', got: "+out)
}

func TestRunCmd_CommandNotFound(t *testing.T) {
	_, err := runCmd("__no_such_command_xyz__")
	testutil.ExpectedError(t, err)
}

func TestRunCmd_NonZeroExit(t *testing.T) {
	_, err := runCmd("false")
	testutil.ExpectedError(t, err)
}
