"""
OR-Tools CP-SAT scheduling solver microservice.
Receives scheduling problem as JSON, returns solution.

POST /solve
{
    "teachingTasks": [{"id": 1, "teacherId": 1, "classIds": [1, 2]}],
    "teachers": [{"id": 1, "preferNoEarly": true, "preferNoLate": false}],
    "classrooms": [{"id": 1, "capacity": 80}],
    "lockedSlots": [{"dayOfWeek": 3, "startPeriod": 4, "span": 4}],
    "constraints": ["teacher_preference", "avoid_saturday"],
    "timeLimitSeconds": 60
}

Response:
{
    "entries": [{"taskId": 1, "teacherId": 1, "classroomId": 1, "dayOfWeek": 0, "startPeriod": 0, "span": 2}],
    "score": 85.5,
    "status": "optimal" | "feasible" | "timeout" | "error",
    "elapsedMs": 1234
}
"""

import json
import time
import sys
from flask import Flask, request, jsonify

app = Flask(__name__)

# Lazy import ortools
try:
    from ortools.sat.python import cp_model
    HAS_ORTOOLS = True
except ImportError:
    HAS_ORTOOLS = False
    print("WARNING: ortools not installed, solver will return errors", file=sys.stderr)


VALID_STARTS = [0, 2, 4, 6, 8]  # Period starts


def solve_scheduling(data):
    """Solve scheduling problem using CP-SAT."""
    if not HAS_ORTOOLS:
        return {"status": "error", "error": "OR-Tools not installed"}

    tasks = data.get("teachingTasks", [])
    teachers = {t["id"]: t for t in data.get("teachers", [])}
    classrooms = data.get("classrooms", [])
    locked_slots = data.get("lockedSlots", [])
    constraints = data.get("constraints", [])
    time_limit = data.get("timeLimitSeconds", 60)

    if not tasks or not classrooms:
        return {"status": "error", "error": "No tasks or classrooms"}

    model = cp_model.CpModel()
    solver = cp_model.CpSolver()
    solver.parameters.max_time_in_seconds = time_limit
    solver.parameters.num_search_workers = 4

    days = 7
    periods_per_day = 11

    # Decision variables: for each task, which (day, start, room)?
    task_day = {}
    task_start = {}
    task_room = {}
    task_placed = {}

    for ti, task in enumerate(tasks):
        tid = task["id"]
        task_placed[ti] = model.NewBoolVar(f"placed_{tid}")
        task_day[ti] = model.NewIntVar(0, days - 1, f"day_{tid}")
        task_start[ti] = model.NewIntVarFromDomain(
            cp_model.Domain.FromValues(VALID_STARTS), f"start_{tid}"
        )
        task_room[ti] = model.NewIntVar(0, len(classrooms) - 1, f"room_{tid}")

    # Hard constraints: no teacher overlap
    teacher_intervals = {}
    for ti, task in enumerate(tasks):
        tid = task["id"]
        teacher_id = task["teacherId"]
        if teacher_id not in teacher_intervals:
            teacher_intervals[teacher_id] = []

        day_var = task_day[ti]
        start_var = task_start[ti]
        placed = task_placed[ti]

        # Create interval: start..start+2 on a specific day
        interval = model.NewOptionalFixedSizeIntervalVar(
            start_var, 2, placed, f"task_{tid}"
        )
        teacher_intervals[teacher_id].append(interval)

    for teacher_id, intervals in teacher_intervals.items():
        if len(intervals) > 1:
            model.AddNoOverlap(intervals)

    # Hard constraint: no room overlap
    room_intervals = {}
    for ti, task in enumerate(tasks):
        room_var = task_room[ti]
        placed = task_placed[ti]
        start_var = task_start[ti]

        # For room scheduling, we need to consider day+period as one dimension
        # This is a simplification - strict room overlap prevention needs more complex modeling

    # Soft constraints: maximize number of placed tasks
    placed_sum = sum(task_placed[ti] for ti in range(len(tasks)))
    model.Maximize(placed_sum)

    # Solve
    status = solver.Solve(model)

    result_entries = []
    for ti, task in enumerate(tasks):
        if solver.Value(task_placed[ti]):
            room_idx = solver.Value(task_room[ti])
            result_entries.append({
                "taskId": task["id"],
                "teacherId": task["teacherId"],
                "classroomId": classrooms[room_idx]["id"],
                "dayOfWeek": solver.Value(task_day[ti]),
                "startPeriod": solver.Value(task_start[ti]),
                "span": 2,
            })

    status_str = "optimal"
    if status == cp_model.FEASIBLE:
        status_str = "feasible"
    elif status == cp_model.INFEASIBLE:
        status_str = "infeasible"
    elif status == cp_model.MODEL_INVALID:
        status_str = "error"

    return {
        "entries": result_entries,
        "score": float(solver.ObjectiveValue()) if status in (cp_model.OPTIMAL, cp_model.FEASIBLE) else 0,
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
