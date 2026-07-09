"""
OR-Tools CP-SAT scheduling solver — X variable model.

Decision variables X[i][r][p]:
  X[i][r][p] = 1 if teaching task i is assigned to room r at position p,
           = 0 otherwise.

  where p = dayOfWeek * PERIODS_PER_DAY + startPeriod
  startPeriod in {0, 2, 4, 6, 8}, span = 2 per task.

Hard constraints (expressed in X):
  HC1 — Teacher no-overlap: Σ_{i of same teacher, r} X[i][r][p] ≤ 1 for each p
  HC2 — Room no-overlap:    Σ_i X[i][r][p] ≤ 1 for each r, p
  HC3 — Class no-overlap:   Σ_{i containing class, r} X[i][r][p] ≤ 1 for each class, p
  HC4 — Locked slots:       X[i][r][p] = 0 if p overlaps any locked slot
  HC5 — Room capacity:      X[i][r][p] = 0 if room[r].capacity < total_students[i]
  HC6 — Teacher unavailable: X[i][r][p] = 0 if p overlaps teacher's unavailable slot

Soft constraints (objective terms):
  SC1 — Teacher preference (early/late avoidance)
  SC2 — Course dispersion (spread sessions across days)
  SC3 — Teacher days limit
  SC4 — Low floor preference
  SC5 — Weekend avoidance
  SC6 — Sports period preference
  SC7 — Student fatigue (excessive same-day tasks)

POST /solve
Input:  ORToolsInput JSON (same format as before)
Output: ORToolsOutput JSON (same format)
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
SPAN = 2  # each course occupies 2 consecutive periods

# Valid positions (p = day * PERIODS_PER_DAY + start)
VALID_POSITIONS = [d * PERIODS_PER_DAY + s for d in range(DAYS) for s in VALID_STARTS]


def periods_overlap(s1, span1, s2, span2):
    """Check if two time spans overlap."""
    return s1 < s2 + span2 and s2 < s1 + span1


def solve_scheduling(data):
    """Solve scheduling using CP-SAT with X[i][r][p] binary variable model."""
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

    # Total students per task (for capacity constraint)
    task_total_students = []
    for cids in task_class_ids:
        total = sum(class_size_map.get(cid, 0) for cid in cids)
        task_total_students.append(total)

    model = cp_model.CpModel()
    solver = cp_model.CpSolver()
    solver.parameters.max_time_in_seconds = time_limit
    solver.parameters.num_search_workers = 4

    # ===== Decision Variables: X[i][r][p] (binary) =====
    X = {}
    for i in range(n_tasks):
        for r in range(n_rooms):
            for p in VALID_POSITIONS:
                X[(i, r, p)] = model.NewBoolVar(f"X_{i}_{r}_{p}")

    # ===== Helper Variables (derived from X) =====
    # placed[i] = Σ_{r,p} X[i][r][p] (each task placed at most once)
    placed = [model.NewBoolVar(f"placed_{i}") for i in range(n_tasks)]
    for i in range(n_tasks):
        x_terms = [X[(i, r, p)] for r in range(n_rooms) for p in VALID_POSITIONS]
        model.Add(sum(x_terms) >= placed[i]).OnlyEnforceIf(placed[i])
        model.Add(sum(x_terms) == 0).OnlyEnforceIf(placed[i].Not())
        model.Add(sum(x_terms) <= 1)

    # room_idx[i] = which room task i is assigned to
    room_idx = [model.NewIntVar(0, max(0, n_rooms - 1), f"room_{i}") for i in range(n_tasks)]
    for i in range(n_tasks):
        for r in range(n_rooms):
            placed_at_r = model.NewBoolVar(f"p_{i}_r_{r}")
            model.Add(sum(X[(i, r, p)] for p in VALID_POSITIONS) >= placed_at_r)
            model.Add(sum(X[(i, r, p)] for p in VALID_POSITIONS) <= placed_at_r * n_tasks)
            model.Add(room_idx[i] == r).OnlyEnforceIf(placed_at_r)

    # position[i] = which position p task i is assigned to
    position = [model.NewIntVar(0, max(VALID_POSITIONS), f"pos_{i}") for i in range(n_tasks)]
    for i in range(n_tasks):
        for p in VALID_POSITIONS:
            at_pos = model.NewBoolVar(f"pos_{i}_{p}")
            model.Add(sum(X[(i, r, p)] for r in range(n_rooms)) >= at_pos)
            model.Add(sum(X[(i, r, p)] for r in range(n_rooms)) <= at_pos * n_tasks)
            model.Add(position[i] == p).OnlyEnforceIf(at_pos)

    # day[i], start[i] derived from position[i]
    # day[i], start[i] derived from position[i] via linear equation
    # Note: CP-SAT supports IntVar * constant but NOT IntVar // constant
    day = [model.NewIntVar(0, DAYS - 1, f"day_{i}") for i in range(n_tasks)]
    start = [model.NewIntVarFromDomain(cp_model.Domain.FromValues(VALID_STARTS), f"start_{i}")
             for i in range(n_tasks)]
    for i in range(n_tasks):
        model.Add(position[i] == day[i] * PERIODS_PER_DAY + start[i])

    # =====================================================================
    # HARD CONSTRAINTS (expressed in X)
    # =====================================================================

    # HC1 — Teacher no-overlap: at each position, at most one task per teacher
    teacher_task_map = {}
    for i, tid in enumerate(task_teacher_ids):
        teacher_task_map.setdefault(tid, []).append(i)

    for _, task_indices in teacher_task_map.items():
        for p in VALID_POSITIONS:
            model.Add(sum(X[(i, r, p)] for i in task_indices for r in range(n_rooms)) <= 1)

    # HC2 — Room no-overlap: at each position, at most one task per room
    for r in range(n_rooms):
        for p in VALID_POSITIONS:
            model.Add(sum(X[(i, r, p)] for i in range(n_tasks)) <= 1)

    # HC3 — Class group no-overlap
    all_class_ids = set()
    for cids in task_class_ids:
        all_class_ids.update(cids)

    class_task_map = {}
    for cid in all_class_ids:
        class_task_map[cid] = [i for i, cids in enumerate(task_class_ids) if cid in cids]

    for cid, task_indices in class_task_map.items():
        for p in VALID_POSITIONS:
            model.Add(sum(X[(i, r, p)] for i in task_indices for r in range(n_rooms)) <= 1)

    # HC4 — Locked slots: forbid overlapping positions
    for i in range(n_tasks):
        for p in VALID_POSITIONS:
            p_day = p // PERIODS_PER_DAY
            p_start = p % PERIODS_PER_DAY
            for ls in locked_slots:
                if int(ls["dayOfWeek"]) == p_day and periods_overlap(p_start, SPAN, int(ls["startPeriod"]), ls["span"]):
                    for r in range(n_rooms):
                        model.Add(X[(i, r, p)] == 0)
                    break

    # HC5 — Room capacity
    for i, total_students in enumerate(task_total_students):
        if total_students <= 0:
            continue
        for r, room in enumerate(classrooms):
            if room["capacity"] < total_students:
                for p in VALID_POSITIONS:
                    model.Add(X[(i, r, p)] == 0)

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
            for p in VALID_POSITIONS:
                p_day = p // PERIODS_PER_DAY
                p_start = p % PERIODS_PER_DAY
                if us_day == p_day and periods_overlap(p_start, SPAN, us_start, us_span):
                    for r in range(n_rooms):
                        model.Add(X[(i, r, p)] == 0)

    # =====================================================================
    # SOFT CONSTRAINTS (objective terms)
    # =====================================================================

    objective_terms = []

    # Base reward for placing a task
    BASE_REWARD = 1000
    for i in range(n_tasks):
        objective_terms.append(placed[i] * BASE_REWARD)

    # SC1 — Teacher preference (avoid early/late)
    if "teacher_preference" in constraints:
        w = weights.get("teacher_preference", 50)
        for i, tid in enumerate(task_teacher_ids):
            teacher = teacher_map.get(tid)
            if not teacher:
                continue
            if teacher.get("preferNoEarly"):
                is_early = model.NewBoolVar(f"early_{i}")
                model.Add(start[i] <= 1).OnlyEnforceIf(is_early)
                model.Add(start[i] > 1).OnlyEnforceIf(is_early.Not())
                active_early = model.NewBoolVar(f"ae_{i}")
                model.AddBoolAnd([placed[i], is_early]).OnlyEnforceIf(active_early)
                model.AddBoolOr([placed[i].Not(), is_early.Not()]).OnlyEnforceIf(active_early.Not())
                objective_terms.append(active_early * (-w))

            if teacher.get("preferNoLate"):
                is_late = model.NewBoolVar(f"late_{i}")
                model.Add(start[i] >= 6).OnlyEnforceIf(is_late)
                model.Add(start[i] < 6).OnlyEnforceIf(is_late.Not())
                active_late = model.NewBoolVar(f"al_{i}")
                model.AddBoolAnd([placed[i], is_late]).OnlyEnforceIf(active_late)
                model.AddBoolOr([placed[i].Not(), is_late.Not()]).OnlyEnforceIf(active_late.Not())
                objective_terms.append(active_late * (-w))

    # SC2 — Weekend avoidance
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

    # SC3 — Sports period preference
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

    # SC4 — Course spacing (maximize unique days per course)
    if "course_dispersed" in constraints:
        w = weights.get("course_dispersed", 50)
        course_task_indices = {}
        for i, cid in enumerate(task_course_ids):
            course_task_indices.setdefault(cid, []).append(i)

        for cid, indices in course_task_indices.items():
            if len(indices) <= 1:
                continue
            for d in range(DAYS):
                any_on_day = model.NewBoolVar(f"c_{cid}_d{d}")
                day_indicators = []
                for i in indices:
                    is_on_day = model.NewBoolVar(f"c_{cid}_t{i}_d{d}")
                    model.Add(day[i] == d).OnlyEnforceIf(is_on_day)
                    model.Add(day[i] != d).OnlyEnforceIf(is_on_day.Not())
                    active_on_day = model.NewBoolVar(f"c_{cid}_t{i}_act_d{d}")
                    model.AddBoolAnd([placed[i], is_on_day]).OnlyEnforceIf(active_on_day)
                    model.AddBoolOr([placed[i].Not(), is_on_day.Not()]).OnlyEnforceIf(active_on_day.Not())
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
                for r_idx, room in enumerate(classrooms):
                    floor = room["floor"]
                    if floor <= 1:
                        continue
                    normalized = int(w * (floor - 1) / (max_floor - 1))
                    if normalized <= 0:
                        continue
                    at_room = model.NewBoolVar(f"lf_{i}_r{r_idx}")
                    model.Add(sum(X[(i, r_idx, p)] for p in VALID_POSITIONS) >= at_room)
                    model.Add(sum(X[(i, r_idx, p)] for p in VALID_POSITIONS) <= at_room * n_tasks)
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
                    is_on_day = model.NewBoolVar(f"t_{i}_d{d}")
                    model.Add(day[i] == d).OnlyEnforceIf(is_on_day)
                    model.Add(day[i] != d).OnlyEnforceIf(is_on_day.Not())
                    active_on_day = model.NewBoolVar(f"t_{i}_act_d{d}")
                    model.AddBoolAnd([placed[i], is_on_day]).OnlyEnforceIf(active_on_day)
                    model.AddBoolOr([placed[i].Not(), is_on_day.Not()]).OnlyEnforceIf(active_on_day.Not())
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

    # SC7 — Student fatigue (penalize when a class group has 3+ tasks on the same day)
    if "student_fatigue" in constraints and all_class_ids:
        w = weights.get("student_fatigue", 50)
        for cid in all_class_ids:
            task_indices = class_task_map[cid]
            if len(task_indices) < 3:
                continue
            for d in range(DAYS):
                any_this_day = []
                for i in task_indices:
                    is_this_day = model.NewBoolVar(f"fat_{cid}_t{i}_d{d}")
                    model.Add(day[i] == d).OnlyEnforceIf(is_this_day)
                    model.Add(day[i] != d).OnlyEnforceIf(is_this_day.Not())
                    active_today = model.NewBoolVar(f"fat_{cid}_t{i}_act_d{d}")
                    model.AddBoolAnd([placed[i], is_this_day]).OnlyEnforceIf(active_today)
                    model.AddBoolOr([placed[i].Not(), is_this_day.Not()]).OnlyEnforceIf(active_today.Not())
                    any_this_day.append(active_today)

                if len(any_this_day) >= 3:
                    count = sum(any_this_day)
                    # Penalty only when count > 2 (3+ tasks on same day)
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
    for i, t in enumerate(tasks):
        if solver.Value(placed[i]):
            r_idx = solver.Value(room_idx[i])
            result_entries.append({
                "taskId": t["id"],
                "teacherId": t["teacherId"],
                "classroomId": classrooms[r_idx]["id"],
                "dayOfWeek": solver.Value(day[i]),
                "startPeriod": solver.Value(start[i]),
                "span": SPAN,
            })

    status_str = "optimal"
    if status == cp_model.FEASIBLE:
        status_str = "feasible"
    elif status == cp_model.INFEASIBLE:
        status_str = "infeasible"
    elif status == cp_model.MODEL_INVALID:
        status_str = "error"

    # Normalize score to 0-100
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
