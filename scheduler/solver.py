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

VALID_STARTS = [0, 2, 4, 6, 8]  # course start periods (legacy — kept for compat)
PERIODS_PER_DAY = 11
DAYS = 7
SPAN = 2  # default span (legacy) — v0.5.1 tasks may override per-session via sessionSpans.

# Valid positions (p = day * PERIODS_PER_DAY + start)
VALID_POSITIONS = [d * PERIODS_PER_DAY + s for d in range(DAYS) for s in VALID_STARTS]


# v0.5.1: HBUT block-alignment rules. MUST stay in sync with
# backend/models/types.go IsSpanLegal + ValidStartsForSpan.
#
#   Morning   (period 0-3) : span ∈ {2}, start ∈ {0, 2}
#   Afternoon (period 4-7) : span ∈ {2}, start ∈ {4, 6}
#   Evening   (period 8-10): span ∈ {1, 2, 3}, any start within block
def is_span_legal(start, span):
    if span < 1 or span > 3:
        return False
    if start < 0 or start + span > PERIODS_PER_DAY:
        return False
    block_start = _block_of(start)
    block_end = _block_of(start + span - 1)
    if block_start is None or block_start != block_end:
        return False
    if block_start == "morning":
        return span == 2 and start in (0, 2)
    if block_start == "afternoon":
        return span == 2 and start in (4, 6)
    if block_start == "evening":
        return True
    return False


def _block_of(period):
    if 0 <= period <= 3:
        return "morning"
    if 4 <= period <= 7:
        return "afternoon"
    if 8 <= period <= 10:
        return "evening"
    return None


def valid_starts_for_span(span):
    return [s for s in range(PERIODS_PER_DAY) if is_span_legal(s, span)]


def periods_overlap(s1, span1, s2, span2):
    """Check if two time spans overlap."""
    return s1 < s2 + span2 and s2 < s1 + span1


def default_spans_from_hours(weekly_hours):
    """v0.5.1: mirror of Go planFromWeeklyHours. Returns list of session spans."""
    if weekly_hours <= 0:
        return [SPAN]
    table = {
        1: [1],
        2: [2],
        3: [3],
        4: [2, 2],
        5: [2, 2, 1],
        6: [2, 2, 2],
        7: [2, 2, 2, 1],
    }
    return table.get(weekly_hours, [2, 2, 2, 2])


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

    # Sessions per task and per-session span (v0.5.1).
    # Go authoritatively fills sessionSpans; if absent, we fall back to hour-derived defaults.
    task_sessions = []
    task_span_list = []  # task_span_list[i] = list of int spans, length == task_sessions[i]
    for t in tasks:
        provided_spans = t.get("sessionSpans") or []
        if provided_spans:
            spans = [int(x) for x in provided_spans[:4] if 1 <= int(x) <= 3]
            if not spans:
                spans = [2]
        else:
            start_week = t.get("startWeek", 1)
            end_week = t.get("endWeek", 16)
            num_weeks = max(1, end_week - start_week + 1)
            total_hours = t.get("totalHours", 0)
            if total_hours > 0:
                weekly_hours = int(round(total_hours / float(num_weeks)))
                if weekly_hours < 1:
                    weekly_hours = 1
                mp = t.get("maxHoursPerWeek", 0)
                if mp > 0 and weekly_hours > mp:
                    weekly_hours = mp
                spans = default_spans_from_hours(weekly_hours)
            else:
                spans = [SPAN]
        # Cap 4 sessions per week (matches Go behavior)
        spans = spans[:4]
        task_sessions.append(len(spans))
        task_span_list.append(spans)

    # Required room type per task
    task_room_type = [t.get("requiredRoomType", "") for t in tasks]

    # Per-(task, session) valid positions: only positions legal for THIS session's span.
    # This replaces the global VALID_POSITIONS for variable generation.
    task_valid_positions = []
    for i in range(n_tasks):
        per_session = []
        for s in range(task_sessions[i]):
            span_s = task_span_list[i][s]
            legal_starts = valid_starts_for_span(span_s)
            positions = [d * PERIODS_PER_DAY + st for d in range(DAYS) for st in legal_starts]
            per_session.append(positions)
        task_valid_positions.append(per_session)

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
    # v0.5.1: p is constrained to positions legal for this session's span.
    X = {}
    for i in range(n_tasks):
        n_sess = task_sessions[i]
        for s in range(n_sess):
            for r in range(n_rooms):
                for p in task_valid_positions[i][s]:
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
            x_terms = [X[(i, s, r, p)] for r in range(n_rooms) for p in task_valid_positions[i][s]]
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
            positions_is = task_valid_positions[i][s]
            for r in range(n_rooms):
                pr = model.NewBoolVar(f"pr_{i}_{s}_{r}")
                model.Add(sum(X[(i, s, r, p)] for p in positions_is) >= pr)
                model.Add(sum(X[(i, s, r, p)] for p in positions_is) <= pr * n_rooms)
                model.Add(room_idx[(i, s)] == r).OnlyEnforceIf(pr)

    # day[i][s], start[i][s], position[i][s] for each session
    day = {}
    start = {}
    position = {}
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            span_s = task_span_list[i][s]
            legal_starts = valid_starts_for_span(span_s)
            positions_is = task_valid_positions[i][s]
            day[(i, s)] = model.NewIntVar(0, DAYS - 1, f"day_{i}_{s}")
            start[(i, s)] = model.NewIntVarFromDomain(
                cp_model.Domain.FromValues(legal_starts if legal_starts else [0]),
                f"start_{i}_{s}"
            )
            max_pos = max(positions_is) if positions_is else 0
            position[(i, s)] = model.NewIntVar(0, max_pos, f"pos_{i}_{s}")
            model.Add(position[(i, s)] == day[(i, s)] * PERIODS_PER_DAY + start[(i, s)])

            # Link position to X variables
            for p in positions_is:
                at_pos = model.NewBoolVar(f"ap_{i}_{s}_{p}")
                model.Add(sum(X[(i, s, r, p)] for r in range(n_rooms)) >= at_pos)
                model.Add(sum(X[(i, s, r, p)] for r in range(n_rooms)) <= at_pos * n_rooms)
                model.Add(position[(i, s)] == p).OnlyEnforceIf(at_pos)

    # =====================================================================
    # HARD CONSTRAINTS
    # =====================================================================

    # v0.5.1: absolute period q for coverage-based hard constraints.
    ABS_PERIODS = DAYS * PERIODS_PER_DAY

    def _covers(i, s, p):
        span_s = task_span_list[i][s]
        return range(p, p + span_s)

    # HC1 — Teacher no-overlap (absolute period coverage)
    teacher_task_map = {}
    for i, tid in enumerate(task_teacher_ids):
        teacher_task_map.setdefault(tid, []).append(i)

    for _, task_indices in teacher_task_map.items():
        for q in range(ABS_PERIODS):
            terms = []
            for i in task_indices:
                for s in range(task_sessions[i]):
                    for p in task_valid_positions[i][s]:
                        if q in _covers(i, s, p):
                            for r in range(n_rooms):
                                terms.append(X[(i, s, r, p)])
            if terms:
                model.Add(sum(terms) <= 1)

    # HC2 — Room no-overlap (skip shared venues like 体育馆; coverage-based)
    for r, room in enumerate(classrooms):
        if room.get("type") == "体育馆":
            continue
        for q in range(ABS_PERIODS):
            terms = []
            for i in range(n_tasks):
                for s in range(task_sessions[i]):
                    for p in task_valid_positions[i][s]:
                        if q in _covers(i, s, p):
                            terms.append(X[(i, s, r, p)])
            if terms:
                model.Add(sum(terms) <= 1)

    # HC3 — Class group no-overlap
    all_class_ids = set()
    for cids in task_class_ids:
        all_class_ids.update(cids)

    class_task_map = {}
    for cid in all_class_ids:
        class_task_map[cid] = [i for i, cids in enumerate(task_class_ids) if cid in cids]

    for cid, task_indices in class_task_map.items():
        for q in range(ABS_PERIODS):
            terms = []
            for i in task_indices:
                for s in range(task_sessions[i]):
                    for p in task_valid_positions[i][s]:
                        if q in _covers(i, s, p):
                            for r in range(n_rooms):
                                terms.append(X[(i, s, r, p)])
            if terms:
                model.Add(sum(terms) <= 1)

    # HC4 — Locked slots (use this session's span)
    for i in range(n_tasks):
        for s in range(task_sessions[i]):
            span_s = task_span_list[i][s]
            for p in task_valid_positions[i][s]:
                p_day = p // PERIODS_PER_DAY
                p_start = p % PERIODS_PER_DAY
                for ls in locked_slots:
                    if int(ls["dayOfWeek"]) == p_day and periods_overlap(p_start, span_s, int(ls["startPeriod"]), ls["span"]):
                        for r in range(n_rooms):
                            model.Add(X[(i, s, r, p)] == 0)
                        break

    # HC5 — Room capacity (skip shared venues like 体育馆)
    for i, total_students in enumerate(task_total_students):
        if total_students <= 0:
            continue
        for r, room in enumerate(classrooms):
            if room.get("type") == "体育馆":
                continue
            if room["capacity"] < total_students:
                for s in range(task_sessions[i]):
                    for p in task_valid_positions[i][s]:
                        model.Add(X[(i, s, r, p)] == 0)

    # HC6 — Teacher unavailable slots (use this session's span)
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
                span_s = task_span_list[i][s]
                for p in task_valid_positions[i][s]:
                    p_day = p // PERIODS_PER_DAY
                    p_start = p % PERIODS_PER_DAY
                    if us_day == p_day and periods_overlap(p_start, span_s, us_start, us_span):
                        for r in range(n_rooms):
                            model.Add(X[(i, s, r, p)] == 0)

    # HC7 — Room type matching
    SPECIALTY_ROOM_TYPES = {"体育馆", "实验室", "机房"}
    for i in range(n_tasks):
        req_type = task_room_type[i]
        for r, room in enumerate(classrooms):
            room_type = room.get("type", "")
            if req_type:
                if room_type and room_type != req_type:
                    for s in range(task_sessions[i]):
                        for p in task_valid_positions[i][s]:
                            model.Add(X[(i, s, r, p)] == 0)
            else:
                if room_type in SPECIALTY_ROOM_TYPES:
                    for s in range(task_sessions[i]):
                        for p in task_valid_positions[i][s]:
                            model.Add(X[(i, s, r, p)] == 0)

    # HC8 — Same-task sessions don't overlap each other (coverage-based)
    for i in range(n_tasks):
        n_sess = task_sessions[i]
        if n_sess <= 1:
            continue
        for q in range(ABS_PERIODS):
            terms = []
            for s in range(n_sess):
                for p in task_valid_positions[i][s]:
                    if q in _covers(i, s, p):
                        for r in range(n_rooms):
                            terms.append(X[(i, s, r, p)])
            if terms:
                model.Add(sum(terms) <= 1)

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

    # SC4 — Course spacing (maximize unique days, penalize consecutive-day clustering)
    if "course_dispersed" in constraints:
        w = weights.get("course_dispersed", 50)
        w_consec = int(w * 0.5)  # consecutive-day penalty weight

        course_task_indices = {}
        for i, cid in enumerate(task_course_ids):
            course_task_indices.setdefault(cid, []).append(i)

        for cid, indices in course_task_indices.items():
            # Collect all sessions across all tasks of this course
            all_sessions = [(i, s) for i in indices for s in range(task_sessions[i])]
            if len(all_sessions) <= 1:
                continue

            any_on_day_vars = []  # collect for consecutive-pair penalties
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
                any_on_day_vars.append(any_on_day)

            # Penalize consecutive-day clustering: discourage Mon+Tue, Tue+Wed, etc.
            for d in range(DAYS - 1):
                consecutive = model.NewBoolVar(f"c_{cid}_consec_d{d}")
                model.AddBoolAnd([any_on_day_vars[d], any_on_day_vars[d + 1]]).OnlyEnforceIf(consecutive)
                model.AddBoolOr([any_on_day_vars[d].Not(), any_on_day_vars[d + 1].Not()]).OnlyEnforceIf(consecutive.Not())
                objective_terms.append(consecutive * (-w_consec))

            # Daily balance: penalize max daily sessions exceeding ideal spread
            w_balance = int(w * 0.2)  # 20% of course_dispersed weight
            ideal_max = max(1, (len(all_sessions) + DAYS - 1) // DAYS)  # ceil(total/DAYS)
            # Count sessions placed on each day
            day_session_counts = []  # IntVars: number of placed sessions per day
            for d in range(DAYS):
                day_count = model.NewIntVar(0, len(all_sessions), f"c_{cid}_dcnt_d{d}")
                day_vars = []
                for (i, s) in all_sessions:
                    is_d = model.NewBoolVar(f"c_{cid}_bal_i{i}_s{s}_d{d}")
                    model.Add(day[(i, s)] == d).OnlyEnforceIf(is_d)
                    model.Add(day[(i, s)] != d).OnlyEnforceIf(is_d.Not())
                    active_d = model.NewBoolVar(f"c_{cid}_bal_a{i}_s{s}_d{d}")
                    model.AddBoolAnd([placed[(i, s)], is_d]).OnlyEnforceIf(active_d)
                    model.AddBoolOr([placed[(i, s)].Not(), is_d.Not()]).OnlyEnforceIf(active_d.Not())
                    day_vars.append(active_d)
                model.Add(day_count == sum(day_vars))
                day_session_counts.append(day_count)
            max_daily = model.NewIntVar(0, len(all_sessions), f"c_{cid}_maxday")
            model.AddMaxEquality(max_daily, day_session_counts)
            excess = model.NewIntVar(0, len(all_sessions), f"c_{cid}_bexcess")
            model.Add(excess >= max_daily - ideal_max)
            objective_terms.append(excess * (-w_balance))

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
                        positions_is = task_valid_positions[i][s]
                        model.Add(sum(X[(i, s, r_idx, p)] for p in positions_is) >= at_room)
                        model.Add(sum(X[(i, s, r_idx, p)] for p in positions_is) <= at_room * n_rooms)
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
                    "span": task_span_list[i][s],
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
                except Exception:
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
        conflicts.append(f"总需求: {total_demand}会话, {n_rooms}教室×{len(VALID_POSITIONS)}时段={n_rooms * len(VALID_POSITIONS)}理论容量")

    # Build unplaced-session diagnostics (separate from constraint conflicts).
    # Triggered when optimal/feasible but not all sessions placed — resource bottleneck.
    unplaced = []
    if sessions_placed < total_sessions_expected:
        unplaced_by_task = {}
        for i, t in enumerate(tasks):
            tid = t.get("id", i)
            for s in range(task_sessions[i]):
                if not solver.Value(placed[(i, s)]):
                    info = unplaced_by_task.setdefault(tid, {
                        "teacherName": teacher_map.get(t["teacherId"], {}).get("name", "?"),
                        "requiredRoomType": task_room_type[i] or "普通教室",
                        "unplacedCount": 0,
                        "totalSessions": task_sessions[i],
                    })
                    info["unplacedCount"] += 1
        for tid in sorted(unplaced_by_task.keys()):
            info = unplaced_by_task[tid]
            unplaced.append(
                f"任务ID={tid} 教师={info['teacherName']} "
                f"需教室={info['requiredRoomType']} "
                f"未排入{info['unplacedCount']}/{info['totalSessions']}个session"
            )

    return {
        "entries": result_entries,
        "score": round(normalized_score, 1),
        "scoreRaw": raw_score,
        "status": status_str,
        "elapsedMs": int(solver.WallTime() * 1000),
        "sessionsPlaced": sessions_placed,
        "sessionsExpected": total_sessions_expected,
        "conflicts": conflicts,
        "unplaced": unplaced,
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
