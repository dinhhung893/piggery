#!/usr/bin/env python3
"""ttydrive.py <envdir> <name> <prompt> : runs the real interactive omp TUI in a pty (isolated env,
probe extension), types <prompt> + Enter, waits until the probe log shows the run settled (agent_end
without willContinue) or RPC_MAX seconds, then quits (Ctrl-C twice / /exit). Raw output ->
/tmp/pg-omp/<name>.tty (ANSI kept), plain text -> /tmp/pg-omp/<name>.txt."""
import os, pty, re, select, subprocess, sys, time, json, fcntl, termios, struct

env_dir, name, prompt = sys.argv[1:4]
max_s = float(os.environ.get("RPC_MAX", "70"))
log = f"/tmp/pg-omp/{name}.jsonl"
try: os.remove(log)
except FileNotFoundError: pass
env = {k: v for k, v in os.environ.items() if k not in ("PIGGERY_DISABLED", "PI_CODING_AGENT")}
env.update(HOME=env_dir, PATH=f"{env_dir}/bin:" + env["PATH"], PI_CODING_AGENT_DIR=f"{env_dir}/agent", PROBE_LOG=log,
           TERM="xterm-256color", COLUMNS="120", LINES="40")
m, s = pty.openpty()
fcntl.ioctl(s, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
proc = subprocess.Popen(["omp", "--model", "HP/glm-5.3-flash", "--thinking", "off"] + os.environ.get("OMP_ARGS", "").split(),
                        cwd=f"{env_dir}/proj", env=env, stdin=s, stdout=s, stderr=s, close_fds=True, start_new_session=True)
os.close(s)
raw = bytearray()

def pump(t):
    end = time.time() + t
    while time.time() < end:
        r, _, _ = select.select([m], [], [], 0.2)
        if r:
            try: d = os.read(m, 65536)
            except OSError: return False
            if not d: return False
            raw.extend(d)
    return True

def events():
    try: return [json.loads(l) for l in open(log)]
    except FileNotFoundError: return []

def settled():
    ev = events()
    ends = [e for e in ev if e["ev"] == "agent_end"]
    return bool(ends) and not ends[-1].get("willContinue") and ev[-1]["ev"] in ("agent_end", "session_shutdown")

pump(5)                                   # startup
os.write(m, b"\r"); pump(3)               # skip the first-run splash
os.write(m, prompt.encode()); pump(0.5); os.write(m, b"\r")
t0 = time.time()
esc_sent = False
while time.time() - t0 < max_s:
    pump(1.0)
    if os.environ.get("TTY_ESC") and not esc_sent and any(e["ev"] == "tool_execution_start" for e in events()):
        pump(1.0); os.write(m, b"\x1b"); esc_sent = True
    if settled(): break
pump(4)                                   # let a steered continuation start, if any
while time.time() - t0 < max_s and not settled(): pump(1.0)
pump(1.5)
os.write(m, b"\x03"); pump(0.6); os.write(m, b"\x03"); pump(1.5)
if proc.poll() is None:
    os.write(m, b"/exit\r"); pump(2)
if proc.poll() is None:
    proc.kill()
open(f"/tmp/pg-omp/{name}.tty", "wb").write(raw)
txt = re.sub(rb"\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b[@-_]", b"", bytes(raw)).decode("utf8", "replace")
open(f"/tmp/pg-omp/{name}.txt", "w").write(txt)
print(f"done rc={proc.poll()} bytes={len(raw)} events={len(events())}")
