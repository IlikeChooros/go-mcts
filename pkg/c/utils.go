package cmcts

/*
#cgo CFLAGS: -I${SRCDIR} -O3
#include "utils.h"
*/
import "C"

func BoolToInt(b bool) int {
	return int(C.boolToInt(C._Bool(b)))
}
