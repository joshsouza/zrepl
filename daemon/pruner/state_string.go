// Code generated by "stringer -type=State"; DO NOT EDIT.

package pruner

import "strconv"

const (
	_State_name_0 = "PlanPlanWait"
	_State_name_1 = "Exec"
	_State_name_2 = "ExecWait"
	_State_name_3 = "ErrPerm"
	_State_name_4 = "Done"
)

var (
	_State_index_0 = [...]uint8{0, 4, 12}
)

func (i State) String() string {
	switch {
	case 1 <= i && i <= 2:
		i -= 1
		return _State_name_0[_State_index_0[i]:_State_index_0[i+1]]
	case i == 4:
		return _State_name_1
	case i == 8:
		return _State_name_2
	case i == 16:
		return _State_name_3
	case i == 32:
		return _State_name_4
	default:
		return "State(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}