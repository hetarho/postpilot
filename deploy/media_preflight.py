#!/usr/bin/env python3
"""Read-only Linux host capacity gate; run on the actual Docker host before rollout."""
import argparse
import json
import math
import os
from pathlib import Path
import shutil
import sys

from media_rollout import memory_bytes, RolloutError


def read_capacity(work_root, proc=Path('/proc'), cgroup=Path('/sys/fs/cgroup')):
    info = dict(line.split(':', 1) for line in (proc / 'meminfo').read_text().splitlines())
    available = int(info['MemAvailable'].split()[0]) * 1024
    memory_limit = cgroup / 'memory.max'
    if memory_limit.exists() and memory_limit.read_text().strip() != 'max':
        available = min(available, max(0, int(memory_limit.read_text()) - int((cgroup / 'memory.current').read_text())))
    cpus = len(os.sched_getaffinity(0)) if hasattr(os, 'sched_getaffinity') else os.cpu_count()
    quota = cgroup / 'cpu.max'
    if quota.exists():
        amount, period = quota.read_text().split()
        if amount != 'max':
            cpus = min(cpus, int(amount) / int(period))
    disk = shutil.disk_usage(work_root).free
    return {'available_memory_bytes': available, 'available_disk_bytes': disk, 'cpu_capacity': cpus}


def assess(capacity, worker_memory, api_extra, reserve, workspace, disk_reserve, worker_cpus):
    required_memory = worker_memory + api_extra + reserve
    required_disk = workspace + disk_reserve
    errors = []
    if not math.isfinite(worker_cpus) or min(worker_memory, reserve, workspace, disk_reserve, worker_cpus) <= 0 or api_extra < 0:
        errors.append('budgets must be positive (API extra may be zero on a worker-only host)')
    if worker_memory < 256 * 1024**2:
        errors.append('worker memory must be at least 256MiB; this floor is not a tested workload guarantee')
    if workspace < 8 * 1024**3:
        errors.append('workspace budget cannot be below the production 8GiB bound')
    if capacity['available_memory_bytes'] < required_memory:
        errors.append('insufficient currently available memory for worker + additional API + reserve')
    if capacity['available_disk_bytes'] < required_disk:
        errors.append('insufficient free disk for one bounded workspace + disk reserve')
    if worker_cpus > capacity['cpu_capacity']:
        errors.append('worker CPU limit exceeds host/cgroup capacity')
    return {**capacity, 'required_memory_bytes': required_memory, 'required_disk_bytes': required_disk,
            'worker_cpus': worker_cpus, 'pass': not errors, 'failures': errors,
            'scope': 'point-in-time headroom; existing workloads already consume MemAvailable; no speed or future-load guarantee'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--work-root', type=Path, required=True, help='existing path on the filesystem holding worker scratch')
    parser.add_argument('--worker-memory', required=True, help='same value as MEDIA_WORKER_MEMORY')
    parser.add_argument('--worker-cpus', type=float, default=1)
    parser.add_argument('--api-extra-memory', default='256m', help='additional API headroom; use 0 on separate worker host')
    parser.add_argument('--reserve-memory', default='256m', help='additional headroom for cohost services beyond present usage')
    parser.add_argument('--workspace', default='8g', help='one production workspace bound; do not lower for arbitrary user inputs')
    parser.add_argument('--reserve-disk', default='2g')
    args = parser.parse_args()
    try:
        if not args.work_root.is_dir():
            raise ValueError('--work-root must be an existing directory on the target filesystem')
        report = assess(read_capacity(args.work_root), *(memory_bytes(v) for v in [args.worker_memory, args.api_extra_memory, args.reserve_memory, args.workspace, args.reserve_disk]), args.worker_cpus)
    except (OSError, KeyError, ValueError, RolloutError) as error:
        parser.exit(2, f'capacity unavailable: {error}; run on the Linux Docker host, not a macOS Docker client\n')
    print(json.dumps(report, indent=2))
    sys.exit(0 if report['pass'] else 1)


if __name__ == '__main__':
    main()
