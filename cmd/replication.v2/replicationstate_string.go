// Code generated by "stringer -type=ReplicationState"; DO NOT EDIT.

package replication

import "strconv"

const (
	_ReplicationState_name_0 = "PlanningPlanningError"
	_ReplicationState_name_1 = "Working"
	_ReplicationState_name_2 = "WorkingWait"
	_ReplicationState_name_3 = "Completed"
	_ReplicationState_name_4 = "ContextDone"
)

var (
	_ReplicationState_index_0 = [...]uint8{0, 8, 21}
)

func (i ReplicationState) String() string {
	switch {
	case 1 <= i && i <= 2:
		i -= 1
		return _ReplicationState_name_0[_ReplicationState_index_0[i]:_ReplicationState_index_0[i+1]]
	case i == 4:
		return _ReplicationState_name_1
	case i == 8:
		return _ReplicationState_name_2
	case i == 16:
		return _ReplicationState_name_3
	case i == 32:
		return _ReplicationState_name_4
	default:
		return "ReplicationState(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}