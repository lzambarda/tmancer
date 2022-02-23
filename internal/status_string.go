// Code generated by "stringer -type=Status"; DO NOT EDIT.

package internal

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Undefined-0]
	_ = x[Close-1]
	_ = x[Opening-2]
	_ = x[Open-3]
	_ = x[Error-4]
	_ = x[Reopening-5]
	_ = x[PortBusy-6]
	_ = x[Signal-7]
	_ = x[Cooper-8]
}

const _Status_name = "UndefinedCloseOpeningOpenErrorReopeningPortBusySignalCooper"

var _Status_index = [...]uint8{0, 9, 14, 21, 25, 30, 39, 47, 53, 59}

func (i Status) String() string {
	if i < 0 || i >= Status(len(_Status_index)-1) {
		return "Status(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Status_name[_Status_index[i]:_Status_index[i+1]]
}
