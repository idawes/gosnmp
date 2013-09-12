package snmp_go

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// A bunch of MIB-2 common oids.
var (
	SYS_DESCR_OID     = []uint32{1, 3, 5, 1, 2, 1, 1, 1}
	SYS_OBJECT_ID_OID = []uint32{1, 3, 5, 1, 2, 1, 1, 2}
	SYS_UPTIME_OID    = []uint32{1, 3, 5, 1, 2, 1, 1, 3}
	SYS_CONTACT_OID   = []uint32{1, 3, 5, 1, 2, 1, 1, 4}
	SYS_NAME_OID      = []uint32{1, 3, 5, 1, 2, 1, 1, 5}
	SYS_LOCATION_OID  = []uint32{1, 3, 5, 1, 2, 1, 1, 6}
)

func parseOid(oidString string) (oid []int, err error) {
	ids := strings.Split(oidString, ".")
	if len(ids) < 2 {
		return nil, errors.New(fmt.Sprintf("The object identifer \"%s\" doesn't contain at least 2 sub identifiers", oidString))
	}
	oid = make([]int, len(ids))
	for i := 0; i < len(ids); i++ {
		if oid[i], err = strconv.Atoi(ids[i]); err != nil {
			return nil, errors.New(fmt.Sprintf("Sub identifier %d in \"%s\" couldn't be parsed", i+1, oidString))
		}
	}
	return oid, nil
}
