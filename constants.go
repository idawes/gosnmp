package gosnmp

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

type snmpBlockType byte

const (
	snmpBlockType_INTEGER           snmpBlockType = 0x02
	snmpBlockType_BIT_STRING                      = 0x03
	snmpBlockType_OCTET_STRING                    = 0x04
	snmpBlockType_NULL                            = 0x05
	snmpBlockType_OBJECT_IDENTIFIER               = 0x06
	snmpBlockType_SEQUENCE                        = 0x30
	snmpBlockType_IP_ADDRESS                      = 0x40
	snmpBlockType_COUNTER_32                      = 0x41
	snmpBlockType_GAUGE_32                        = 0x42
	snmpBlockType_TIME_TICKS                      = 0x43
	snmpBlockType_OPAQUE                          = 0x44
	snmpBlockType_NSAP_ADDRESS                    = 0x45
	snmpBlockType_COUNTER_64                      = 0x46
	snmpBlockTYpe_UINT_32                         = 0x47
)
