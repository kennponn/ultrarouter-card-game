"""Render review evidence from actual CI artifacts; images go to a separate repo.

Usage: python render_evidence.py ARTIFACT_DIR DATA_DIR EXTERNAL_IMAGE_DIR
Requires matplotlib and numpy. No game saves or account databases are published.
"""
import csv
import hashlib
import json
import sys
from collections import Counter
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

source, data_dir, image_dir = map(Path, sys.argv[1:])
data_dir.mkdir(parents=True, exist_ok=True)
image_dir.mkdir(parents=True, exist_ok=True)

def read(name):
    return json.loads((source / name).read_text(encoding="utf-8"))

def save(name, value):
    (data_dir / name).write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

def figure(name, fig):
    fig.savefig(image_dir / name, dpi=170, bbox_inches="tight", facecolor="#f8fafc")
    plt.close(fig)

plt.rcParams.update({"font.family": "DejaVu Sans", "axes.spines.top": False,
                     "axes.spines.right": False, "axes.facecolor": "#f8fafc",
                     "figure.facecolor": "#f8fafc", "font.size": 11,
                     "axes.titleweight": "bold", "axes.labelcolor": "#334155"})

summary = read("summary.json")
save("go-summary.json", summary)
for phase in ("baseline", "candidate"):
    save(phase + "-source-hashes.json", read(phase + "-source-hashes.json"))
    nodes = {}
    for line in (source / (phase + "-tests.jsonl")).read_text(encoding="utf-8").splitlines():
        event = json.loads(line)
        if event.get("Test") and event["Action"] in ("pass", "fail", "skip"):
            nodes[event["Package"] + "/" + event["Test"]] = event["Action"]
    save(phase + "-test-nodes.json", nodes)

samples = read("actual-guild-combat.json")
with (data_dir / "actual-combat.csv").open("w", newline="", encoding="utf-8") as stream:
    writer = csv.DictWriter(stream, fieldnames=list(samples[0]))
    writer.writeheader()
    for item in samples:
        writer.writerow({**item, "squad": " ".join(item["squad"])})

hp = [1300000, 8500000, 42000000, 135000000, 360000000]
stats = []
for tier, stage, strength in sorted({(s["tier"], s["stage"], s["strength"]) for s in samples}):
    group = [s for s in samples if (s["tier"], s["stage"], s["strength"]) == (tier, stage, strength)]
    damage = np.array([s["damage"] for s in group])
    p10, median, p90 = np.percentile(damage, [10, 50, 90])
    stats.append(dict(tier=tier, stage=stage, strength=strength, samples=len(group),
                      members=group[0]["members"], hp=hp[stage-1],
                      damage_p10=float(p10), damage_median=float(median), damage_p90=float(p90),
                      damage_mean=float(damage.mean()), min_rounds=min(s["rounds"] for s in group),
                      max_rounds=max(s["rounds"] for s in group),
                      days_full=hp[stage-1]/(group[0]["members"]*2*median),
                      days_attendance80=hp[stage-1]/(group[0]["members"]*2*0.8*median)))
save("combat-statistics.json", {"reports": len(samples), "events_checked": sum(s["events"] for s in samples),
                              "method": "Percentiles from unchanged Go combat; days = full HP / (members * 2 attempts * attendance * median damage). Days are an estimate, not a live guild observation.",
                              "cells": stats})

fig, ax = plt.subplots(figsize=(10, 4.8))
for strength, offset, color in (("weak", -.23, "#94a3b8"), ("medium", 0, "#2563eb"), ("strong", .23, "#0d9488")):
    rows = [s for s in stats if s["tier"] == s["stage"] and s["strength"] == strength]
    y = np.array([s["damage_median"] for s in rows])
    ax.errorbar(np.arange(1, 6)+offset, y,
                yerr=[y-np.array([s["damage_p10"] for s in rows]), np.array([s["damage_p90"] for s in rows])-y],
                fmt="o", capsize=5, color=color, label=strength.title())
ax.set(xticks=range(1, 6), xlabel="Boss stage at its matching growth tier", ylabel="Damage per challenge (log scale)",
       title="Actual Go combat: median and P10-P90 damage")
ax.set_yscale("log")
ax.grid(axis="y", alpha=.2)
ax.legend(ncols=3)
figure("13-boss-damage.png", fig)

fig, ax = plt.subplots(figsize=(10, 4.8))
ax.axhspan(2, 3, color="#dcfce7", label="2-3 day target")
for strength, color in (("weak", "#64748b"), ("medium", "#2563eb"), ("strong", "#0d9488")):
    rows = [s for s in stats if s["tier"] == s["stage"] and s["strength"] == strength]
    values = [s["days_attendance80"] for s in rows]
    ax.plot(range(1, 6), values, "o-", color=color, label=strength.title())
    if strength == "medium":
        for x, y in enumerate(values, 1): ax.annotate(f"{y:.2f}d", (x, y), xytext=(0, 9), textcoords="offset points", ha="center", color=color)
ax.set(xticks=range(1, 6), xlabel="Boss stage / active members: 8, 15, 22, 30, 38", ylabel="Estimated days",
       title="Guild pace sensitivity: 80% attendance, 2 attempts/day", ylim=(0, 5.5))
ax.legend(ncols=4, loc="upper left")
ax.grid(axis="y", alpha=.2)
figure("14-boss-days.png", fig)

upgrade = []
for engine in ("sqlite", "mysql", "postgres"):
    corpus = read(engine + "-legacy-corpus.json")
    accounts = corpus["accounts"]
    upgrade.append(dict(engine=engine, accounts=len(accounts), guilds=sum(a["save"].get("guild") is not None for a in accounts),
                        unresolved_runs=sum(bool(a.get("run_id")) for a in accounts), old_gift_attachments=len(accounts),
                        serialized_asset_fields=14,
                        corpus_sha256=hashlib.sha256((source/(engine+"-legacy-corpus.json")).read_bytes()).hexdigest()))
save("upgrade-statistics.json", upgrade)
fig, ax = plt.subplots(figsize=(10, 4.8))
for key, label, offset, color in (("accounts", "Accounts / inventories / mail", -.25, "#2563eb"),
                                 ("guilds", "Damaged legacy guilds", 0, "#0d9488"),
                                 ("unresolved_runs", "Unsettled legacy battles", .25, "#d97706")):
    bars = ax.bar(np.arange(3)+offset, [u[key] for u in upgrade], .23, label=label, color=color)
    ax.bar_label(bars, padding=3)
ax.set(xticks=range(3), xticklabels=["SQLite", "MySQL", "PostgreSQL"], ylabel="Actual persisted cases", ylim=(0, 120),
       title="Same database upgrade: old executable writes, new executable reads")
ax.legend(loc="upper right")
figure("15-database-upgrade.png", fig)

fig, ax = plt.subplots(figsize=(10, 4.8))
labels = [s["revision"] for s in summary]
for name, color, offset in (("passed", "#0d9488", -.23), ("failed", "#dc2626", 0), ("skipped", "#94a3b8", .23)):
    bars = ax.bar(np.arange(len(summary))+offset, [s[name] for s in summary], .22, label=name.title(), color=color)
    ax.bar_label(bars, padding=3, fontsize=9)
ax.set(xticks=range(len(summary)), xticklabels=labels, ylabel="Named Go test nodes (includes subtests)",
       title="Executed Go checks by revision and experiment")
ax.tick_params(axis="x", labelsize=9)
ax.legend()
ax.margins(y=.2)
figure("16-go-checks.png", fig)
save("artifact-manifest.json", {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(source.iterdir()) if p.is_file() and p.suffix != ".sqlite"})
print(json.dumps({"reports": len(samples), "combat_cells": len(stats), "events": sum(s["events"] for s in samples), "upgrade_accounts": sum(u["accounts"] for u in upgrade)}))
