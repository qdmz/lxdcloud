package admin

import (
	"strconv"
)

func parseID(s string, v *uint) (int, error) {
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	*v = uint(id)
	return 1, nil
}
