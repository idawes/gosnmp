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
	snmpBlockType_UINT_32                         = 0x47
	snmpBlockType_NO_SUCH_OBJECT                  = 0x80
	snmpBlockType_NO_SUCH_INSTANCE                = 0x81
	snmpBlockType_END_OF_MIB_VIEW                 = 0x82
)

type snmpRequestErrorType int32

const (
	snmpRequestErrorType_NO_ERROR             snmpRequestErrorType = 0
	snmpRequestErrorType_TOO_BIG                                   = 1
	snmpRequestErrorType_NO_SUCH_NAME                              = 2
	snmpRequestErrorType_BAD_VALUE                                 = 3
	snmpRequestErrorType_READ_ONLY                                 = 4
	snmpRequestErrorType_GENERIC_ERROR                             = 5
	snmpRequestErrorType_NO_ACCESS                                 = 6
	snmpRequestErrorType_WRONG_TYPE                                = 7
	snmpRequestErrorType_WRONG_LENGTH                              = 8
	snmpRequestErrorType_WRONG_ENCODING                            = 9
	snmpRequestErrorType_WRONG_VALUE                               = 10
	snmpRequestErrorType_NO_CREATION                               = 11
	snmpRequestErrorType_INCONSISTENT_VALUE                        = 12
	snmpRequestErrorType_RESOURCE_UNAVAILABLE                      = 13
	snmpRequestErrorType_COMMIT_FAILED                             = 14
	snmpRequestErrorType_UNDO_FAILED                               = 15
	snmpRequestErrorType_AUTHORIZATION_ERROR                       = 16
	snmpRequestErrorType_NOT_WRITABLE                              = 17
	snmpRequestErrorType_INCONSISTENT_NAME                         = 18
)
