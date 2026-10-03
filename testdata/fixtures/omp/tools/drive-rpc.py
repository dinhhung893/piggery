#!/usr/bin/env python3
"""rpcdrive.py <envdir> <name> <scenario> [prompt]
Runs `omp --mode rpc` isolated in <envdir> (own HOME/agent dir, probe extension loaded), sends the
prompt, and per scenario: plain (wait for the session to settle), abort (send abort when the first
bash tool starts), then closes stdin. Frames -> /tmp/pg-omp/<name>.frames.jsonl, probe events ->
/tmp/pg-omp/<name>.jsonl (PROBE_LOG). PROBE_* env vars pass through."""
import json, os, subprocess, sys, threading, time

env_dir, name, scenario = sys.argv[1:4]
prompt = sys.argv[4] if len(sys.argv) > 4 else "Reply with exactly the word: one"
settle_s = float(os.environ.get("RPC_SETTLE", "6"))       # quiet time after the last frame that ends the run
max_s = float(os.environ.get("RPC_MAX", "90"))
frames_path = f"/tmp/pg-omp/{name}.frames.jsonl"
for p in (frames_path, f"/tmp/pg-omp/{name}.jsonl"):
    try: os.remove(p)
    except FileNotFoundError: pass

env = {k: v for k, v in os.environ.items() if k not in ("PIGGERY_DISABLED", "PI_CODING_AGENT")}
env.update(HOME=env_dir, PATH=f"{env_dir}/bin:" + env["PATH"], PI_CODING_AGENT_DIR=f"{env_dir}/agent",
           PROBE_LOG=f"/tmp/pg-omp/{name}.jsonl")
args = ["omp", "--mode", "rpc", "--model", "HP/glm-5.3-flash", "--thinking", "off"] + os.environ.get("OMP_ARGS", "").split()
proc = subprocess.Popen(args, cwd=f"{env_dir}/proj", env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                        stderr=open(f"/tmp/pg-omp/{name}.stderr", "w"), text=True, bufsize=1)
t0 = time.time()
frames = open(frames_path, "w")
prompts = [prompt] + [x for x in os.environ.get("RPC_MORE", "").split("|") if x]
state = {"results": 0, "last": time.time(), "settled": False, "bash_started": False, "sent_abort": False, "streaming": False, "ended": 0}

def send(obj):
    proc.stdin.write(json.dumps(obj) + "\n"); proc.stdin.flush()

def reader():
    for line in proc.stdout:
        try: f = json.loads(line)
        except Exception: continue
        f = {"t": round(time.time() - t0, 3), **f}
        # keep frames small: drop big message bodies
        def cut(v, d=0):
            if isinstance(v, str): return v if len(v) <= 160 else v[:160] + "…"
            if isinstance(v, list): return [cut(x, d + 1) for x in v[:4]] + (["…%d more" % (len(v) - 4)] if len(v) > 4 else [])
            if isinstance(v, dict): return {k: (cut(x, d + 1) if d < 4 else "{…}") for k, x in v.items()}
            return v
        if ty_ := f.get("type") == "message_update": state["streaming"] = True
        if f.get("type") != "message_update":
            frames.write(json.dumps(cut(f)) + "\n"); frames.flush()
        state["last"] = time.time()
        ty = f.get("type")
        if ty == "prompt_result": state["results"] += 1
        if ty == "session_settled": state["settled"] = True
        if ty == "agent_end": state["ended"] += 1
        if ty == "tool_execution_start" and f.get("toolName") == "bash": state["bash_started"] = True

threading.Thread(target=reader, daemon=True).start()
# wait for the ready frame
for _ in range(100):
    time.sleep(0.1)
    if os.path.getsize(frames_path) > 0: break
send({"id": "p1", "type": "prompt", "message": prompts[0]})
sent = 1
end = time.time() + max_s
while time.time() < end and proc.poll() is None:
    time.sleep(0.2)
    if scenario == "abort_stream" and state["streaming"] and not state["sent_abort"]:
        time.sleep(0.4); send({"id": "a1", "type": "abort"}); state["sent_abort"] = True
    if scenario == "abort" and state["bash_started"] and not state["sent_abort"]:
        time.sleep(1.0); send({"id": "a1", "type": "abort"}); state["sent_abort"] = True
    if sent < len(prompts) and state["results"] >= sent and time.time() - state["last"] > 2:
        state["settled"] = False; sent += 1; send({"id": f"p{sent}", "type": "prompt", "message": prompts[sent - 1]}); continue
    if sent < len(prompts): continue
    if os.environ.get("RPC_AFTER") and state["settled"] and time.time() - state["last"] > 2 and not state.get("after"):
        state["after"] = True; state["settled"] = False; state["last"] = time.time(); send(json.loads(os.environ["RPC_AFTER"])); continue
    if state["settled"] and time.time() - state["last"] > 1.5 and (not os.environ.get("RPC_AFTER") or state.get("after")): break
    if state["ended"] and time.time() - state["last"] > settle_s: break
try: proc.stdin.close()
except Exception: pass
try: proc.wait(timeout=15)
except subprocess.TimeoutExpired: proc.kill()
print(f"done rc={proc.returncode} settled={state['settled']} agent_end_frames={state['ended']} abort_sent={state['sent_abort']}")
