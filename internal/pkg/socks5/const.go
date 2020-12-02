package socks5

const (
	verSocks5 = 0x05

	authNoAuthenticationRequired      = 0x00
	authUsernamePassword              = 0x02
	authUsernamePasswordVersion       = 0x01
	authUsernamePasswordStatusSuccess = 0x00
	authUsernamePasswordStatusFailure = 0x01
	authNoAcceptableMethods           = 0xFF

	cmdConnect      = 0x01
	cmdBind         = 0x02
	cmdUDPAssociate = 0x03

	atypIPv4Address = 0x01
	atypDomainName  = 0x03
	atypIPv6Address = 0x04

	repSucceeded                     = 0x00
	repGeneralSocksServerFailure     = 0x01
	repConnectionNotAllowedByRuleset = 0x02
	repNetworkUnreachable            = 0x03
	repHostUnreachable               = 0x04
	repConnectionRefused             = 0x05
	repTTLExpired                    = 0x06
	repComandNotSupported            = 0x07
	repAddressTypeNotSupported       = 0x08

	rsvReserved = 0x00
)
