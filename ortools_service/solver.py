"""
OR-Tools CP-SAT scheduling solver microservice.
Implements full hard + soft constraint system mirroring the SA solver.

Hard constraints (must satisfy):
  - Teacher time conflict (no teacher in two places at once)
  - Classroom occupancy conflict
  - Class group conflict (including combined classes)
  - Locked time slot avoidance

Soft constraints (weighted, contribute to objective):
  - teacher_preference: avoid early (period 0-1) / late (period 6+)
  - course_dispersed: spread same course's sessions across days
  - teacher_days_limit: max days per week per teacher
  - low_floor_preference: prefer lower floors
  - avoid_saturday / avoid_sunday: weekend avoidance
  - pe_preferred_periods: sports courses at start=2 or start=6

POST /solve
{
    "teachingTasks": [{"id": 1, "teacherId": 1, "courseId": 1, "classIds": [1, 2]}],
    "teachers": [{"id": 1, "preferNoEarly": true, "preferNoLate": false, "maxDaysPerWeek": 3, "preferLowFloor": true}],
    "classrooms": [{"id": 1, "floor": 1, "capacity": 80}],
    "classGroups": [{"id": 1, "students": 86}, {"id": 2, "students": 82}],
    "lockedSlots": [{"dayOfWeek": 3, "startPeriod": 4, "span": 4}],
    "constraints": ["teacher_preference", "avoid_saturday"],
    "constraintWeights": {"teacher_preference": 50, "avoid_saturday": 30},
    "sportsCourseIDs": [1, 5],
    "timeLimitSeconds": 60
}
"""

import json
import sys
import time
from flask import Flask, request, jsonify

app = Flask(__name__)

try:
    from ortools.sat.python import cp_model
    HAS_ORTOOLS = True
except ImportError:
    HAS_ORTOOLS = False

VALID_STARTS = [0, 2, 4, 6, 8]  # course start periods
PERIODS_PER_DAY = 11
DAYS = 7


def solve_scheduling(data):
    """Solve scheduling problem using CP-SAT with full constraint system."""
    if not HAS_ORTOOLS:
        return {"status": "error", "error": "OR-Tools not installed"}

    tasks = data.get("teachingTasks", [])
    teachers_list = data.get("teachers", [])
    classrooms = data.get("classrooms", [])
    locked_slots = data.get("lockedSlots", [])
    constraints = data.get("constraints", [])
    weights = data.get("constraintWeights", {})
    sports_ids = set(data.get("sportsCourseIDs", []))
    time_limit = data.get("timeLimitSeconds", 60)

    if not tasks or not classrooms:
        return {"status": "error", "error": "No tasks or classrooms"}

    # Build lookup maps
    teacher_map = {t["id"]: t for t in teachers_list}
    n_tasks = len(tasks)
    n_rooms = len(classrooms)

    # Pre-compute class group IDs per task
    task_class_ids = [t.get("classIds", []) for t in tasks]

    model = cp_model.CpModel()
    solver = cp_model.CpSolver()
    solver.parameters.max_time_in_seconds = time_limit
    solver.parameters.num_search_workers = 4

    # ===== Decision Variables =====
    placed = [model.NewBoolVar(f"placed_{i}") for i in range(n_tasks)]
    day = [model.NewIntVar(0, DAYS - 1, f"day_{i}") for i in range(n_tasks)]
    start = [model.NewIntVarFromDomain(cp_model.Domain.FromValues(VALID_STARTS), f"start_{i}")
             for i in range(n_tasks)]
    room_idx = [model.NewIntVar(0, n_rooms - 1, f"room_{i}") for i in range(n_tasks)]

    # Position on a 1D timeline: position = day * PERIODS_PER_DAY + start
    position = [model.NewIntVar(0, DAYS * PERIODS_PER_DAY - 1, f"pos_{i}") for i in range(n_tasks)]
    for i in range(n_tasks):
        model.Add(position[i] == day[i] * PERIODS_PER_DAY + start[i])

    # ===== Hard Constraint: Locked Slots =====
    for i in range(n_tasks):
        for ls in locked_slots:
            ls_day = ls["dayOfWeek"]
            ls_start = ls["startPeriod"]
            ls_end = ls_start + ls["span"]
            # If task is placed on the locked day AND start overlaps, it's invalid
            is_locked_day = model.NewBoolVar(f"locked_day_{i}_{ls_day}_{ls_start}")
            model.Add(day[i] == ls_day).OnlyEnforceIf(is_locked_day)
            model.Add(day[i] != ls_day).OnlyEnforceIf(is_locked_day.Not())

            # Overlap check: start < ls_end AND start+2 > ls_start
            # Equivalent to: NOT (start+2 <= ls_start OR start >= ls_end)
            after_ls = model.NewBoolVar(f"after_ls_{i}_{ls_day}_{ls_start}")
            before_ls = model.NewBoolVar(f"before_ls_{i}_{ls_day}_{ls_start}")
            model.Add(start[i] + 2 <= ls_start).OnlyEnforceIf(after_ls)
            model.Add(start[i] + 2 > ls_start).OnlyEnforceIf(after_ls.Not())
            model.Add(start[i] >= ls_end).OnlyEnforceIf(before_ls)
            model.Add(start[i] < ls_end).OnlyEnforceIf(before_ls.Not())

            # If locked day AND overlap, task cannot be placed
            no_overlap = model.NewBoolVar(f"no_ov_{i}_{ls_day}_{ls_start}")
            model.AddBoolOr([after_ls, before_ls]).OnlyEnforceIf(no_overlap)
            model.AddBoolAnd([after_ls.Not(), before_ls.Not()]).OnlyEnforceIf(no_overlap.Not())

            # is_locked_day AND (not no_overlap) => placed[i] = False
            conflict = model.NewBoolVar(f"conflict_{i}_{ls_day}_{ls_start}")
            model.AddBoolAnd([is_locked_day, no_overlap.Not()]).OnlyEnforceIf(conflict)
            model.AddBoolOr([is_locked_day.Not(), no_overlap]).OnlyEnforceIf(conflict.Not())
            model.AddImplication(conflict, placed[i].Not())

    # ===== Hard Constraint: Teacher No-Overlap =====
    teacher_task_ids = {}
    for i, t in enumerate(tasks):
        tid = t["teacherId"]
        teacher_task_ids.setdefault(tid, []).append(i)

    for tid, task_indices in teacher_task_ids.items():
        intervals = []
        for i in task_indices:
            itv = model.new_optional_interval_var(
                position[i], 2, position[i] + 2, placed[i], f"teacher_{tid}_task_{i}"
            )
            intervals.append(itv)
        model.AddNoOverlap(intervals)

    # ===== Hard Constraint: Room No-Overlap =====
    for r in range(n_rooms):
        room_intervals = []
        for i in range(n_tasks):
            in_room = model.NewBoolVar(f"t{i}_in_r{r}")
            model.Add(room_idx[i] == r).OnlyEnforceIf(in_room)
            model.Add(room_idx[i] != r).OnlyEnforceIf(in_room.Not())
            active = model.NewBoolVar(f"t{i}_active_r{r}")
            model.AddBoolAnd([placed[i], in_room]).OnlyEnforceIf(active)
            model.AddBoolOr([placed[i].Not(), in_room.Not()]).OnlyEnforceIf(active.Not())
            room_intervals.append(
                model.new_optional_interval_var(
                    position[i], 2, position[i] + 2, active, f"t{i}_r{r}"
                )
            )
        model.AddNoOverlap(room_intervals)

    # ===== Hard Constraint: Class Group No-Overlap =====
    all_class_ids = set()
    for cids in task_class_ids:
        all_class_ids.update(cids)

    for cid in all_class_ids:
        class_intervals = []
        for i, cids in enumerate(task_class_ids):
            if cid not in cids:
                continue
            class_intervals.append(model.new_optional_interval_var(
                position[i], 2, position[i] + 2, placed[i], f"class_{cid}_task_{i}"
            ))
        model.AddNoOverlap(class_intervals)

    # ===== Soft Constraints (as objective terms) =====
    objective_terms = []

    # Base reward for placing a task (high enough to outweigh soft penalties)
    BASE_REWARD = 1000
    for i in range(n_tasks):
        objective_terms.append(placed[i] * BASE_REWARD)

    # Soft: Teacher preference (avoid early/late)
    if "teacher_preference" in constraints:
        w = weights.get("teacher_preference", 50)
        for i, t in enumerate(tasks):
            teacher = teacher_map.get(t["teacherId"])
            if not teacher:
                continue
            # Early penalty (start <= 1 means periods 1-2, i.e. 8:20-9:55)
            if teacher.get("preferNoEarly"):
                is_early = model.NewBoolVar(f"early_{i}")
                model.Add(start[i] <= 1).OnlyEnforceIf(is_early)
                model.Add(start[i] > 1).OnlyEnforceIf(is_early.Not())
                early_penalty = model.NewIntVar(0, 1, f"early_p_{i}")
                model.Add(early_penalty == 1).OnlyEnforceIf(is_early)
                model.Add(early_penalty == 0).OnlyEnforceIf(is_early.Not())
                # Only penalize if placed
                actual_penalty = model.NewIntVar(0, w, f"early_ap_{i}")
                model.AddMultiplicationEquality(actual_penalty, [placed[i], model.NewConstant(w)])
                model.Add(actual_penalty <= w).OnlyEnforceIf(is_early)
                model.Add(actual_penalty == 0).OnlyEnforceIf(is_early.Not())
                # Simpler: just add -w if placed AND early
                active_early = model.NewBoolVar(f"ae_{i}")
                model.AddBoolAnd([placed[i], is_early]).OnlyEnforceIf(active_early)
                model.AddBoolOr([placed[i].Not(), is_early.Not()]).OnlyEnforceIf(active_early.Not())
                objective_terms.append(active_early * (-w))

            # Late penalty (start >= 6 means periods 7+, i.e. 15:55+)
            if teacher.get("preferNoLate"):
                is_late = model.NewBoolVar(f"late_{i}")
                model.Add(start[i] >= 6).OnlyEnforceIf(is_late)
                model.Add(start[i] < 6).OnlyEnforceIf(is_late.Not())
                active_late = model.NewBoolVar(f"al_{i}")
                model.AddBoolAnd([placed[i], is_late]).OnlyEnforceIf(active_late)
                model.AddBoolOr([placed[i].Not(), is_late.Not()]).OnlyEnforceIf(active_late.Not())
                objective_terms.append(active_late * (-w))

    # Soft: Weekend avoidance
    avoid_sat = "avoid_saturday" in constraints
    avoid_sun = "avoid_sunday" in constraints
    if avoid_sat or avoid_sun:
        w = weights.get("avoid_saturday", weights.get("avoid_sunday", 30))
        for i in range(n_tasks):
            if avoid_sat:
                is_sat = model.NewBoolVar(f"sat_{i}")
                model.Add(day[i] == 5).OnlyEnforceIf(is_sat)
                model.Add(day[i] != 5).OnlyEnforceIf(is_sat.Not())
                active_sat = model.NewBoolVar(f"as_{i}")
                model.AddBoolAnd([placed[i], is_sat]).OnlyEnforceIf(active_sat)
                model.AddBoolOr([placed[i].Not(), is_sat.Not()]).OnlyEnforceIf(active_sat.Not())
                objective_terms.append(active_sat * (-w))
            if avoid_sun:
                is_sun = model.NewBoolVar(f"sun_{i}")
                model.Add(day[i] == 6).OnlyEnforceIf(is_sun)
                model.Add(day[i] != 6).OnlyEnforceIf(is_sun.Not())
                active_sun = model.NewBoolVar(f"asu_{i}")
                model.AddBoolAnd([placed[i], is_sun]).OnlyEnforceIf(active_sun)
                model.AddBoolOr([placed[i].Not(), is_sun.Not()]).OnlyEnforceIf(active_sun.Not())
                objective_terms.append(active_sun * (-w))

    # Soft: Sports period preference (start=2 or start=6)
    if "pe_preferred_periods" in constraints:
        w = weights.get("pe_preferred_periods", 50)
        for i, t in enumerate(tasks):
            if t.get("courseId") in sports_ids:
                at_preferred = model.NewBoolVar(f"pe_{i}")
                model.AddLinearExpressionInDomain(
                    start[i], cp_model.Domain.FromValues([2, 6])
                ).OnlyEnforceIf(at_preferred)
                model.Add(start[i] != 2).OnlyEnforceIf(at_preferred.Not())
                model.Add(start[i] != 6).OnlyEnforceIf(at_preferred.Not())
                active_pe = model.NewBoolVar(f"ape_{i}")
                model.AddBoolAnd([placed[i], at_preferred]).OnlyEnforceIf(active_pe)
                model.AddBoolOr([placed[i].Not(), at_preferred.Not()]).OnlyEnforceIf(active_pe.Not())
                objective_terms.append(active_pe * w)

    # Soft: Course spacing (maximize unique days covered by each course)
    if "course_dispersed" in constraints:
        w = weights.get("course_dispersed", 50)
        course_task_ids = {}
        for i, t in enumerate(tasks):
            cid = t["courseId"]
            course_task_ids.setdefault(cid, []).append(i)

        for cid, task_indices in course_task_ids.items():
            if len(task_indices) <= 1:
                continue  # single-session courses are already "perfectly spread"
            for d in range(DAYS):
                any_on_day = model.NewBoolVar(f"course_{cid}_day_{d}")
                day_indicators = []
                for i in task_indices:
                    is_on_day = model.NewBoolVar(f"c_{cid}_t{i}_d{d}")
                    model.Add(day[i] == d).OnlyEnforceIf(is_on_day)
                    model.Add(day[i] != d).OnlyEnforceIf(is_on_day.Not())
                    active_on_day = model.NewBoolVar(f"c_{cid}_t{i}_act_d{d}")
                    model.AddBoolAnd([placed[i], is_on_day]).OnlyEnforceIf(active_on_day)
                    model.AddBoolOr([placed[i].Not(), is_on_day.Not()]).OnlyEnforceIf(active_on_day.Not())
                    day_indicators.append(active_on_day)
                # any_on_day ⇔ at least one task of this course is on this day
                model.AddBoolOr(day_indicators).OnlyEnforceIf(any_on_day)
                for ind in day_indicators:
                    model.AddImplication(ind, any_on_day)
                objective_terms.append(any_on_day * w)

    # Soft: Low floor preference (penalize higher floors for teachers who prefer low)
    if "low_floor_preference" in constraints:
        w = weights.get("low_floor_preference", 50)
        max_floor = max(r["floor"] for r in classrooms) if classrooms else 5
        if max_floor > 1:
            for i, t in enumerate(tasks):
                teacher = teacher_map.get(t["teacherId"])
                if not teacher or not teacher.get("preferLowFloor"):
                    continue
                for r_idx, room in enumerate(classrooms):
                    floor = room["floor"]
                    if floor <= 1:
                        continue
                    # normalize: floor 1 = 0 penalty, floor max = w penalty
                    normalized_penalty = int(w * (floor - 1) / (max_floor - 1))
                    if normalized_penalty <= 0:
                        continue
                    in_room = model.NewBoolVar(f"lf_{i}_r{r_idx}")
                    model.Add(room_idx[i] == r_idx).OnlyEnforceIf(in_room)
                    model.Add(room_idx[i] != r_idx).OnlyEnforceIf(in_room.Not())
                    active_floor = model.NewBoolVar(f"lf_act_{i}_r{r_idx}")
                    model.AddBoolAnd([placed[i], in_room]).OnlyEnforceIf(active_floor)
                    model.AddBoolOr([placed[i].Not(), in_room.Not()]).OnlyEnforceIf(active_floor.Not())
                    objective_terms.append(active_floor * (-normalized_penalty))

    # Soft: Teacher days limit (exact: penalize days exceeding maxDaysPerWeek)
    if "teacher_days_limit" in constraints:
        w = weights.get("teacher_days_limit", 50)
        for tid, task_indices in teacher_task_ids.items():
            teacher = teacher_map.get(tid)
            if not teacher:
                continue
            max_days = teacher.get("maxDaysPerWeek", 3)
            # Count distinct days per teacher
            day_used = {}
            for d in range(DAYS):
                any_on_day = model.NewBoolVar(f"tchr_{tid}_day_{d}")
                day_indicators = []
                for i in task_indices:
                    is_on_day = model.NewBoolVar(f"t_{i}_on_{d}")
                    model.Add(day[i] == d).OnlyEnforceIf(is_on_day)
                    model.Add(day[i] != d).OnlyEnforceIf(is_on_day.Not())
                    active_on_day = model.NewBoolVar(f"t_{i}_act_{d}")
                    model.AddBoolAnd([placed[i], is_on_day]).OnlyEnforceIf(active_on_day)
                    model.AddBoolOr([placed[i].Not(), is_on_day.Not()]).OnlyEnforceIf(active_on_day.Not())
                    day_indicators.append(active_on_day)
                # any_on_day ⇔ at least one task is on this day
                model.AddBoolOr(day_indicators).OnlyEnforceIf(any_on_day)
                for ind in day_indicators:
                    model.AddImplication(ind, any_on_day)
                day_used[d] = any_on_day

            # Exact penalty: extra = max(0, total_days - max_days)
            total_days = sum(day_used.values())
            diff = model.NewIntVar(-DAYS, DAYS, f"diff_{tid}")
            model.Add(diff == total_days - max_days)
            extra = model.NewIntVar(0, DAYS, f"extra_{tid}")
            zero = model.NewConstant(0)
            model.AddMaxEquality(extra, [zero, diff])
            objective_terms.append(extra * (-w * 2))

    # ===== Objective =====
    model.Maximize(sum(objective_terms))

    # ===== Solve =====
    status = solver.Solve(model)

    # ===== Extract Result =====
    result_entries = []
    for i, t in enumerate(tasks):
        if solver.Value(placed[i]):
            r_idx = solver.Value(room_idx[i])
            result_entries.append({
                "taskId": t["id"],
                "teacherId": t["teacherId"],
                "classroomId": classrooms[r_idx]["id"],
                "dayOfWeek": solver.Value(day[i]),
                "startPeriod": solver.Value(start[i]),
                "span": 2,
            })

    status_str = "optimal"
    if status == cp_model.FEASIBLE:
        status_str = "feasible"
    elif status == cp_model.INFEASIBLE:
        status_str = "infeasible"
    elif status == cp_model.MODEL_INVALID:
        status_str = "error"

    # Normalize score to 0-100 scale for consistency with Go scoring system
    n_placed = sum(1 for i in range(n_tasks) if solver.Value(placed[i]))
    raw_score = float(solver.ObjectiveValue())
    max_base = n_placed * BASE_REWARD if n_placed > 0 else 1
    normalized_score = max(0.0, min(100.0, raw_score / max_base * 100))

    return {
        "entries": result_entries,
        "score": round(normalized_score, 1),
        "scoreRaw": raw_score,
        "status": status_str,
        "elapsedMs": int(solver.WallTime() * 1000),
    }


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok", "ortools": HAS_ORTOOLS})


@app.route("/solve", methods=["POST"])
def solve():
    try:
        data = request.get_json()
        start = time.time()
        result = solve_scheduling(data)
        result["elapsedMs"] = int((time.time() - start) * 1000)
        return jsonify(result)
    except Exception as e:
        return jsonify({"status": "error", "error": str(e)}), 500


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 19877
    print(f"OR-Tools scheduling service starting on port {port}")
    app.run(host="127.0.0.1", port=port, debug=False)
