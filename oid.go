package snmp_go

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
