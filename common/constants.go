package common

type SnmpVersion int

const (
	Version1  SnmpVersion = 0x00
	Version2c             = 0x01
)

func (version SnmpVersion) String() string {
	switch version {
	case Version1:
		return "SNMPv1"
	case Version2c:
		return "SNMPv2c"
	default:
		return "Unknown"
	}
}

type SnmpBlockType byte

const (
	INTEGER           SnmpBlockType = 0x02
	BIT_STRING                      = 0x03
	OCTET_STRING                    = 0x04
	NULL                            = 0x05
	OBJECT_IDENTIFIER               = 0x06
	SEQUENCE                        = 0x30
	IP_ADDRESS                      = 0x40
	COUNTER_32                      = 0x41
	GAUGE_32                        = 0x42
	TIME_TICKS                      = 0x43
	OPAQUE                          = 0x44
	NSAP_ADDRESS                    = 0x45
	COUNTER_64                      = 0x46
	UINT_32                         = 0x47
)
