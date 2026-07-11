"""
OR-Tools CP-SAT scheduling solver — multi-session X variable model.

Decision variables X[i][s][r][p]:
  X[i][s][r][p] = 1 if session s of task i is assigned to room r at position p,
                  = 0 otherwise.

  where p = dayOfWeek * PERIODS_PER_DAY + startPeriod
  startPeriod in {0, 2, 4, 6, 8}, span = 2 per session.
  s in {0, 1, ..., sessions_per_task[i] - 1}

Hard constraints (expressed in X):
  HC1 — Teacher no-overlap (across all sessions of all tasks)
  HC2 — Room no-overlap
  HC3 — Class group no-overlap (across all sessions)
  HC4 — Locked slots
  HC5 — Room capacity
  HC6 — Teacher unavailable slots
  HC7 — Room type matching (course requires specific room type)
  HC8 — Same-task sessions don't overlap each other

Soft constraints (objective terms):
  SC1 — Teacher preference (early/late avoidance)
  SC2 — Course dispersion (spread sessions across days)
  SC3 — Teacher days limit
  SC4 — Low floor preference
  SC5 — Weekend avoidance
  SC6 — Sports period preference
  SC7 — Student fatigue (excessive same-day tasks)

POST /solve
Input:  ORToolsInput JSON
Output: ORToolsOutput JSON
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
SPAN = 2  # each session occupies 2 consecutive periods

# Valid positions (p = day * PERIODS_PER_DAY + start)
VALID_POSITIONS = [d * PERIODS_PER_DAY + s for d in range(DAYS) for s in VALID_STARTS]


def periods_overlap(s1, span1, s2, span2):
    """Check if two time spans overlap."""
    return s1 < s2 + span2 and s2 < s1 + span1


def calc_sessions_per_week(total_hours, num_weeks, max_per_week=0):
    """Calculate weekly sessions from total course hours and actual week span."""
    if total_hours <= 0:
        return 1
    if num_weeks <= 0:
        num_weeks = 16
    # Each session = 2 periods = 2学时.
    # Weekly hours = total_hours / num_weeks, sessions = ceil(weekly_hours / 2)
    weekly_hours = total_hours / float(num_weeks)
    sessions = int(weekly_hours / 2.0 + 0.999)  # ceil
    if sessions < 1:
        sessions = 1
    if max_per_week > 0:
        max_sessions = max(1, max_per_week // 2)
        sessions = min(sessions, max_sessions)
    return min(sessions, 4)  # cap at 4 sessions/week


def solve_scheduling(data):
    """Solve scheduling using CP-SAT with multi-session X[i][s][r][p] variable model."""
    if not HAS_ORTOOLS:
        return {"status": "error", "error": "OR-Tools not installed"}

    tasks = data.get("teachingTasks", [])
    teachers_list = data.get("teachers", [])
    classrooms = data.get("classrooms", [])
    class_groups = data.get("classGroups", [])
    locked_slots = data.get("lockedSlots", [])
    constraints = data.get("constraints", [])
    weights = data.get("constraintWeights", {})
    sports_ids = set(data.get("sportsCourseIDs", []))
    time_limit = data.get("timeLimitSeconds", 60)

    if not tasks or not classrooms:
        return {"status": "error", "error": "No tasks or classrooms"}

    # Build lookups
    teacher_map = {t["id"]: t for t in teachers_list}
    class_size_map = {cg["id"]: cg.get("students", 0) for cg in class_groups}
    n_tasks = len(tasks)
    n_rooms = len(classrooms)

    # Pre-compute task data
    task_class_ids = [t.get("classIds", []) for t in tasks]
    task_teacher_ids = [t["teacherId"] for t in tasks]
    task_course_ids = [t["courseId"] for t in tasks]

    # Sessions per task (from input or computed from totalHours with actual week span)
    task_sessions = []
    for t in tasks:
        spw = t.get("sessionsPerWeek", 0)
        if spw <= 0:
            start_week = t.get("startWeek", 1)
            end_week = t.get("endWeek", 16)
            num_weeks = end_week - start_week + 1
            spw = calc_sessions_per_week(
                t.get("totalHours", 0),
                num_weeks,
                t.get("maxHoursPerWeek", 0)
            )
        task_sessions.append(max(1, min(spw, 4)))

    # Required room type per task
    task_room_type = [t.get("requiredRoomType", "") for t in tasks]

    # Total students per task (for capacity constraint)
    task_total_students = []
    for cids in task_class_ids:
        total = sum(class_size_map.get(cid, 0) for cid in cids)
        task_total_students.append(total)

    model = cp_model.CpModel()
    solver = cp_model.CpSolver()
    solver.parameters.max_time_in_seconds = time_limit
    solver.parameters.num_search_workers = 4

    # ===== Decision Variables: X[i][s][r][p] (binary) =====
    X = {}
    for i in range(n_tasks):
        n_sess = task_sessions[i]
        for s in range(n_sess):
            for r in range(n_rooms):
                for p in VALID_POSITIONS:
                    X[(i, s, r, p)] = model.NewBoolVar(f"X_{i}_{s}_{r}_{p}")

    # ===== Helper Variables =====
    # placed[i][s] = session s of task i is placed
    placed = {}
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            placed[(i, s)] = model.NewBoolVar(f"placed_{i}_{s}")

    # Each session placed at most once
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            x_terms = [X[(i, s, r, p)] for r in range(n_rooms) for p in VALID_POSITIONS]
            model.Add(sum(x_terms) >= placed[(i, s)]).OnlyEnforceIf(placed[(i, s)])
            model.Add(sum(x_terms) == 0).OnlyEnforceIf(placed[(i, s)].Not())
            model.Add(sum(x_terms) <= 1)

    # all_placed[i] = all sessions of task i are placed
    all_placed = [model.NewBoolVar(f"all_placed_{i}") for i in range(n_tasks)]
    for i in range(n_tasks):
        n_sess = task_sessions[i]
        for s in range(n_sess):
            model.AddImplication(all_placed[i], placed[(i, s)])
            model.AddImplication(placed[(i, s)].Not(), all_placed[i].Not())

    # room_idx[i][s] = which room session s of task i uses
    room_idx = {}
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            room_idx[(i, s)] = model.NewIntVar(0, max(0, n_rooms - 1), f"room_{i}_{s}")
            for r in range(n_rooms):
                pr = model.NewBoolVar(f"pr_{i}_{s}_{r}")
                model.Add(sum(X[(i, s, r, p)] for p in VALID_POSITIONS) >= pr)
                model.Add(sum(X[(i, s, r, p)] for p in VALID_POSITIONS) <= pr * n_rooms)
                model.Add(room_idx[(i, s)] == r).OnlyEnforceIf(pr)

    # day[i][s], start[i][s], position[i][s] for each session
    day = {}
    start = {}
    position = {}
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            day[(i, s)] = model.NewIntVar(0, DAYS - 1, f"day_{i}_{s}")
            start[(i, s)] = model.NewIntVarFromDomain(
                cp_model.Domain.FromValues(VALID_STARTS), f"start_{i}_{s}"
            )
            position[(i, s)] = model.NewIntVar(0, max(VALID_POSITIONS), f"pos_{i}_{s}")
            model.Add(position[(i, s)] == day[(i, s)] * PERIODS_PER_DAY + start[(i, s)])

            # Link position to X variables
            for p in VALID_POSITIONS:
                at_pos = model.NewBoolVar(f"ap_{i}_{s}_{p}")
                model.Add(sum(X[(i, s, r, p)] for r in range(n_rooms)) >= at_pos)
                model.Add(sum(X[(i, s, r, p)] for r in range(n_rooms)) <= at_pos * n_rooms)
                model.Add(position[(i, s)] == p).OnlyEnforceIf(at_pos)

    # =====================================================================
    # HARD CONSTRAINTS
    # =====================================================================

    # HC1 — Teacher no-overlap (across all sessions of all tasks)
    teacher_task_map = {}
    for i, tid in enumerate(task_teacher_ids):
        teacher_task_map.setdefault(tid, []).append(i)

    for _, task_indices in teacher_task_map.items():
        for p in VALID_POSITIONS:
            model.Add(sum(
                X[(i, s, r, p)]
                for i in task_indices
                for s in range(task_sessions[i])
                for r in range(n_rooms)
            ) <= 1)

    # HC2 — Room no-overlap (skip shared venues like 体育馆)
    for r, room in enumerate(classrooms):
        if room.get("type") == "体育馆":
            continue  # shared venues can host multiple classes simultaneously
        for p in VALID_POSITIONS:
            model.Add(sum(
                X[(i, s, r, p)]
                for i in range(n_tasks)
                for s in range(task_sessions[i])
            ) <= 1)

    # HC3 — Class group no-overlap
    all_class_ids = set()
    for cids in task_class_ids:
        all_class_ids.update(cids)

    class_task_map = {}
    for cid in all_class_ids:
        class_task_map[cid] = [i for i, cids in enumerate(task_class_ids) if cid in cids]

    for cid, task_indices in class_task_map.items():
        for p in VALID_POSITIONS:
            model.Add(sum(
                X[(i, s, r, p)]
                for i in task_indices
                for s in range(task_sessions[i])
                for r in range(n_rooms)
            ) <= 1)

    # HC4 — Locked slots
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            for p in VALID_POSITIONS:
                p_day = p // PERIODS_PER_DAY
                p_start = p % PERIODS_PER_DAY
                for ls in locked_slots:
                    if int(ls["dayOfWeek"]) == p_day and periods_overlap(p_start, SPAN, int(ls["startPeriod"]), ls["span"]):
                        for r in range(n_rooms):
                            model.Add(X[(i, s, r, p)] == 0)
                        break

    # HC5 — Room capacity (skip shared venues like 体育馆)
    for i, total_students in enumerate(task_total_students):
        if total_students <= 0:
            continue
        for r, room in enumerate(classrooms):
            if room.get("type") == "体育馆":
                continue  # sports venues have unlimited effective capacity
            if room["capacity"] < total_students:
                for s in range(task_sessions[i]):
                    for p in VALID_POSITIONS:
                        model.Add(X[(i, s, r, p)] == 0)

    # HC6 — Teacher unavailable slots
    for i, tid in enumerate(task_teacher_ids):
        teacher = teacher_map.get(tid, {})
        unavail_slots = teacher.get("unavailableSlots", "")
        if not unavail_slots:
            continue
        if isinstance(unavail_slots, str):
            try:
                unavail_slots = json.loads(unavail_slots)
            except (json.JSONDecodeError, TypeError):
                continue
        if not isinstance(unavail_slots, list):
            continue
        for us in unavail_slots:
            us_day = us.get("dayOfWeek", -1)
            us_start = us.get("startPeriod", 0)
            us_span = us.get("span", 2)
            for s in range(task_sessions[i]):
                for p in VALID_POSITIONS:
                    p_day = p // PERIODS_PER_DAY
                    p_start = p % PERIODS_PER_DAY
                    if us_day == p_day and periods_overlap(p_start, SPAN, us_start, us_span):
                        for r in range(n_rooms):
                            model.Add(X[(i, s, r, p)] == 0)

    # HC7 — Room type matching
    for i in range(n_tasks):
        req_type = task_room_type[i]
        if not req_type:
            continue
        for r, room in enumerate(classrooms):
            room_type = room.get("type", "")
            if room_type and room_type != req_type:
                for s in range(task_sessions[i]):
                    for p in VALID_POSITIONS:
                        model.Add(X[(i, s, r, p)] == 0)

    # HC8 — Same-task sessions don't overlap each other
    for i in range(n_tasks):
        n_sess = task_sessions[i]
        if n_sess <= 1:
            continue
        for p in VALID_POSITIONS:
            model.Add(sum(
                X[(i, s, r, p)]
                for s in range(n_sess)
                for r in range(n_rooms)
            ) <= 1)

    # =====================================================================
    # SOFT CONSTRAINTS (objective terms)
    # =====================================================================

    objective_terms = []

    # Base reward for placing sessions
    BASE_REWARD = 1000
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            objective_terms.append(placed[(i, s)] * BASE_REWARD)
        # Extra bonus for all sessions placed
        objective_terms.append(all_placed[i] * 500)

    # SC1 — Teacher preference (avoid early/late)
    if "teacher_preference" in constraints:
        w = weights.get("teacher_preference", 50)
        for i, tid in enumerate(task_teacher_ids):
            teacher = teacher_map.get(tid)
            if not teacher:
                continue
            for s in range(task_sessions[i]):
                if teacher.get("preferNoEarly"):
                    is_early = model.NewBoolVar(f"early_{i}_{s}")
                    model.Add(start[(i, s)] <= 1).OnlyEnforceIf(is_early)
                    model.Add(start[(i, s)] > 1).OnlyEnforceIf(is_early.Not())
                    active_early = model.NewBoolVar(f"ae_{i}_{s}")
                    model.AddBoolAnd([placed[(i, s)], is_early]).OnlyEnforceIf(active_early)
                    model.AddBoolOr([placed[(i, s)].Not(), is_early.Not()]).OnlyEnforceIf(active_early.Not())
                    objective_terms.append(active_early * (-w))

                if teacher.get("preferNoLate"):
                    is_late = model.NewBoolVar(f"late_{i}_{s}")
                    model.Add(start[(i, s)] >= 6).OnlyEnforceIf(is_late)
                    model.Add(start[(i, s)] < 6).OnlyEnforceIf(is_late.Not())
                    active_late = model.NewBoolVar(f"al_{i}_{s}")
                    model.AddBoolAnd([placed[(i, s)], is_late]).OnlyEnforceIf(active_late)
                    model.AddBoolOr([placed[(i, s)].Not(), is_late.Not()]).OnlyEnforceIf(active_late.Not())
                    objective_terms.append(active_late * (-w))

    # SC2 — Weekend avoidance
    avoid_sat = "avoid_saturday" in constraints
    avoid_sun = "avoid_sunday" in constraints
    if avoid_sat or avoid_sun:
        w_sat = weights.get("avoid_saturday", 30)
        w_sun = weights.get("avoid_sunday", 30)
        for i in range(n_tasks):
            for s in range(task_sessions[i]):
                if avoid_sat:
                    is_sat = model.NewBoolVar(f"sat_{i}_{s}")
                    model.Add(day[(i, s)] == 5).OnlyEnforceIf(is_sat)
                    model.Add(day[(i, s)] != 5).OnlyEnforceIf(is_sat.Not())
                    active_sat = model.NewBoolVar(f"as_{i}_{s}")
                    model.AddBoolAnd([placed[(i, s)], is_sat]).OnlyEnforceIf(active_sat)
                    model.AddBoolOr([placed[(i, s)].Not(), is_sat.Not()]).OnlyEnforceIf(active_sat.Not())
                    objective_terms.append(active_sat * (-w_sat))
                if avoid_sun:
                    is_sun = model.NewBoolVar(f"sun_{i}_{s}")
                    model.Add(day[(i, s)] == 6).OnlyEnforceIf(is_sun)
                    model.Add(day[(i, s)] != 6).OnlyEnforceIf(is_sun.Not())
                    active_sun = model.NewBoolVar(f"asu_{i}_{s}")
                    model.AddBoolAnd([placed[(i, s)], is_sun]).OnlyEnforceIf(active_sun)
                    model.AddBoolOr([placed[(i, s)].Not(), is_sun.Not()]).OnlyEnforceIf(active_sun.Not())
                    objective_terms.append(active_sun * (-w_sun))

    # SC3 — Sports period preference
    if "pe_preferred_periods" in constraints:
        w = weights.get("pe_preferred_periods", 50)
        for i, t in enumerate(tasks):
            if t.get("courseId") in sports_ids:
                for s in range(task_sessions[i]):
                    at_preferred = model.NewBoolVar(f"pe_{i}_{s}")
                    model.AddLinearExpressionInDomain(
                        start[(i, s)], cp_model.Domain.FromValues([2, 6])
                    ).OnlyEnforceIf(at_preferred)
                    model.Add(start[(i, s)] != 2).OnlyEnforceIf(at_preferred.Not())
                    model.Add(start[(i, s)] != 6).OnlyEnforceIf(at_preferred.Not())
                    active_pe = model.NewBoolVar(f"ape_{i}_{s}")
                    model.AddBoolAnd([placed[(i, s)], at_preferred]).OnlyEnforceIf(active_pe)
                    model.AddBoolOr([placed[(i, s)].Not(), at_preferred.Not()]).OnlyEnforceIf(active_pe.Not())
                    objective_terms.append(active_pe * w)

    # SC4 — Course spacing (maximize unique days per course across all sessions)
    if "course_dispersed" in constraints:
        w = weights.get("course_dispersed", 50)
        course_task_indices = {}
        for i, cid in enumerate(task_course_ids):
            course_task_indices.setdefault(cid, []).append(i)

        for cid, indices in course_task_indices.items():
            # Collect all sessions across all tasks of this course
            all_sessions = [(i, s) for i in indices for s in range(task_sessions[i])]
            if len(all_sessions) <= 1:
                continue
            for d in range(DAYS):
                any_on_day = model.NewBoolVar(f"c_{cid}_d{d}")
                day_indicators = []
                for (i, s) in all_sessions:
                    is_on_day = model.NewBoolVar(f"c_{cid}_t{i}_s{s}_d{d}")
                    model.Add(day[(i, s)] == d).OnlyEnforceIf(is_on_day)
                    model.Add(day[(i, s)] != d).OnlyEnforceIf(is_on_day.Not())
                    active_on_day = model.NewBoolVar(f"c_{cid}_t{i}_s{s}_act_d{d}")
                    model.AddBoolAnd([placed[(i, s)], is_on_day]).OnlyEnforceIf(active_on_day)
                    model.AddBoolOr([placed[(i, s)].Not(), is_on_day.Not()]).OnlyEnforceIf(active_on_day.Not())
                    day_indicators.append(active_on_day)
                model.AddBoolOr(day_indicators).OnlyEnforceIf(any_on_day)
                for ind in day_indicators:
                    model.AddImplication(ind, any_on_day)
                objective_terms.append(any_on_day * w)

    # SC5 — Low floor preference
    if "low_floor_preference" in constraints:
        w = weights.get("low_floor_preference", 50)
        max_floor = max(r["floor"] for r in classrooms) if classrooms else 5
        if max_floor > 1:
            for i, t in enumerate(tasks):
                teacher = teacher_map.get(t["teacherId"])
                if not teacher or not teacher.get("preferLowFloor"):
                    continue
                for s in range(task_sessions[i]):
                    for r_idx, room in enumerate(classrooms):
                        floor = room["floor"]
                        if floor <= 1:
                            continue
                        normalized = int(w * (floor - 1) / (max_floor - 1))
                        if normalized <= 0:
                            continue
                        at_room = model.NewBoolVar(f"lf_{i}_{s}_r{r_idx}")
                        model.Add(sum(X[(i, s, r_idx, p)] for p in VALID_POSITIONS) >= at_room)
                        model.Add(sum(X[(i, s, r_idx, p)] for p in VALID_POSITIONS) <= at_room * n_rooms)
                        objective_terms.append(at_room * (-normalized))

    # SC6 — Teacher days limit
    if "teacher_days_limit" in constraints:
        w = weights.get("teacher_days_limit", 50)
        for tid, task_indices in teacher_task_map.items():
            teacher = teacher_map.get(tid)
            if not teacher:
                continue
            max_days = teacher.get("maxDaysPerWeek", 3)
            day_used = {}
            for d in range(DAYS):
                any_on_day = model.NewBoolVar(f"td_{tid}_d{d}")
                day_indicators = []
                for i in task_indices:
                    for s in range(task_sessions[i]):
                        is_on_day = model.NewBoolVar(f"t_{i}_s{s}_d{d}")
                        model.Add(day[(i, s)] == d).OnlyEnforceIf(is_on_day)
                        model.Add(day[(i, s)] != d).OnlyEnforceIf(is_on_day.Not())
                        active_on_day = model.NewBoolVar(f"t_{i}_s{s}_act_d{d}")
                        model.AddBoolAnd([placed[(i, s)], is_on_day]).OnlyEnforceIf(active_on_day)
                        model.AddBoolOr([placed[(i, s)].Not(), is_on_day.Not()]).OnlyEnforceIf(active_on_day.Not())
                        day_indicators.append(active_on_day)
                model.AddBoolOr(day_indicators).OnlyEnforceIf(any_on_day)
                for ind in day_indicators:
                    model.AddImplication(ind, any_on_day)
                day_used[d] = any_on_day

            total_days = sum(day_used.values())
            diff = model.NewIntVar(-DAYS, DAYS, f"d_diff_{tid}")
            model.Add(diff == total_days - max_days)
            extra = model.NewIntVar(0, DAYS, f"d_extra_{tid}")
            zero = model.NewConstant(0)
            model.AddMaxEquality(extra, [zero, diff])
            objective_terms.append(extra * (-w * 2))

    # SC7 — Student fatigue (3+ sessions for same class on same day)
    if "student_fatigue" in constraints and all_class_ids:
        w = weights.get("student_fatigue", 50)
        for cid in all_class_ids:
            task_indices = class_task_map[cid]
            all_sessions = [(i, s) for i in task_indices for s in range(task_sessions[i])]
            if len(all_sessions) < 3:
                continue
            for d in range(DAYS):
                any_this_day = []
                for (i, s) in all_sessions:
                    is_this_day = model.NewBoolVar(f"fat_{cid}_t{i}_s{s}_d{d}")
                    model.Add(day[(i, s)] == d).OnlyEnforceIf(is_this_day)
                    model.Add(day[(i, s)] != d).OnlyEnforceIf(is_this_day.Not())
                    active_today = model.NewBoolVar(f"fat_{cid}_t{i}_s{s}_act_d{d}")
                    model.AddBoolAnd([placed[(i, s)], is_this_day]).OnlyEnforceIf(active_today)
                    model.AddBoolOr([placed[(i, s)].Not(), is_this_day.Not()]).OnlyEnforceIf(active_today.Not())
                    any_this_day.append(active_today)

                if len(any_this_day) >= 3:
                    count = sum(any_this_day)
                    has_excess = model.NewBoolVar(f"fat_has_{cid}_d{d}")
                    model.Add(count >= 3).OnlyEnforceIf(has_excess)
                    model.Add(count <= 2).OnlyEnforceIf(has_excess.Not())
                    excess = model.NewIntVar(0, len(any_this_day), f"fat_ex_{cid}_d{d}")
                    model.Add(excess == count - 2).OnlyEnforceIf(has_excess)
                    model.Add(excess == 0).OnlyEnforceIf(has_excess.Not())
                    objective_terms.append(excess * (-w))

    # ===== Objective =====
    model.Maximize(sum(objective_terms))

    # ===== Solve =====
    status = solver.Solve(model)

    # ===== Extract Result =====
    result_entries = []
    total_sessions_expected = sum(task_sessions)
    sessions_placed = 0

    for i, t in enumerate(tasks):
        for s in range(task_sessions[i]):
            if solver.Value(placed[(i, s)]):
                r_idx = solver.Value(room_idx[(i, s)])
                result_entries.append({
                    "taskId": t["id"],
                    "teacherId": t["teacherId"],
                    "classroomId": classrooms[r_idx]["id"],
                    "dayOfWeek": solver.Value(day[(i, s)]),
                    "startPeriod": solver.Value(start[(i, s)]),
                    "span": SPAN,
                    "sessionIndex": s,
                })
                sessions_placed += 1

    status_str = "optimal"
    if status == cp_model.FEASIBLE:
        status_str = "feasible"
    elif status == cp_model.INFEASIBLE:
        status_str = "infeasible"
    elif status == cp_model.MODEL_INVALID:
        status_str = "error"

    # Normalize score to 0-100
    raw_score = 0.0
    if status in (cp_model.OPTIMAL, cp_model.FEASIBLE):
        raw_score = float(solver.ObjectiveValue())
    max_base = sessions_placed * BASE_REWARD if sessions_placed > 0 else 1
    normalized_score = max(0.0, min(100.0, raw_score / max_base * 100))

    # Build conflict diagnostics for infeasible
    conflicts = []
    if status == cp_model.INFEASIBLE:
        # Teacher demand vs supply
        for tid, task_indices in teacher_task_map.items():
            demand = sum(task_sessions[i] for i in task_indices)
            # Available slots = VALID_POSITIONS minus locked and unavailable
            supply = len(VALID_POSITIONS)
            teacher = teacher_map.get(tid, {})
            unavail = teacher.get("unavailableSlots", "")
            if unavail:
                try:
                    unavail_list = json.loads(unavail) if isinstance(unavail, str) else unavail
                    blocked_positions = set()
                    for us in unavail_list:
                        for p in VALID_POSITIONS:
                            p_day = p // PERIODS_PER_DAY
                            p_start = p % PERIODS_PER_DAY
                            if int(us.get("dayOfWeek", -1)) == p_day and periods_overlap(p_start, SPAN, int(us.get("startPeriod", 0)), us.get("span", 2)):
                                blocked_positions.add(p)
                    supply = len(VALID_POSITIONS) - len(blocked_positions)
                except:
                    pass
            if demand > supply:
                conflicts.append(f"教师{teacher.get('name', tid)}: 需排{demand}会话, 可用{supply}时段")

        # Room capacity bottleneck
        for i, total_s in enumerate(task_total_students):
            if total_s <= 0:
                continue
            suitable_rooms = sum(1 for r in classrooms if r["capacity"] >= total_s)
            if suitable_rooms == 0:
                conflicts.append(f"任务{tasks[i].get('id', i)}: 需{total_s}座, 无足够容量教室")

        # Total demand vs total supply
        total_demand = sum(task_sessions)
        total_supply_per_slot = n_rooms
        conflicts.append(f"总需求: {total_demand}会话, {n_rooms}教室×{len(VALID_POSITIONS)}时段={total_demand * len(VALID_POSITIONS)}理论容量")

    return {
        "entries": result_entries,
        "score": round(normalized_score, 1),
        "scoreRaw": raw_score,
        "status": status_str,
        "elapsedMs": int(solver.WallTime() * 1000),
        "sessionsPlaced": sessions_placed,
        "sessionsExpected": total_sessions_expected,
        "conflicts": conflicts,
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
