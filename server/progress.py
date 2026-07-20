"""服务端进度上报器。

把 BaseDownloader 的进度回调（按作品计数 + 字节级下载）实时写到对应的
``DownloadJob`` 上，前端轮询 ``GET /api/v1/jobs`` 即可拿到进度条/网速数据。

无锁：GIL 下 int/str 写原子，进度 UI 允许轻微抖动。
"""

from __future__ import annotations

import time
from typing import Optional

from server.jobs import DownloadJob


class ServerProgressReporter:
    """实现 core.downloader_base.ProgressReporter 协议，写 job 字段。"""

    # 网速采样的最小时间窗口（秒）。chunk 是成簇到达的，逐 chunk 算瞬时速度会把
    # 缓冲区突发读放大成虚高的 MB/s；只在距上次采样 ≥ 该窗口时才重算一次速度，
    # 把突发平均掉，得到贴近真实吞吐的数值。
    _SPEED_SAMPLE_INTERVAL = 0.5

    def __init__(self, job: DownloadJob):
        self._job = job
        # 网速计算的状态：上次采样时间 / 上次采样字节数
        self._last_ts: Optional[float] = None
        self._last_bytes = 0
        # 单文件下载开始时重置字节计数，避免上一个文件的 written 残留
        self._current_filename: Optional[str] = None

    # ---- 按作品计数（协议方法）----
    def update_step(self, step: str, detail: str = "") -> None:
        self._job.current_step = step

    def set_item_total(self, total: int, detail: str = "") -> None:
        self._job.item_total = int(total or 0)

    def advance_item(self, status: str, detail: str = "") -> None:
        self._job.item_done += 1

    def on_author(self, nickname: Optional[str] = None, sec_uid: Optional[str] = None) -> None:
        """预留：服务端暂不缓存作者信息，接口兼容即可。"""

    # ---- 字节级进度（协议扩展方法）----
    def update_bytes(
        self, downloaded: int, total: Optional[int], filename: str = ""
    ) -> None:
        job = self._job
        # 切换到新文件时重置字节基准，保证百分比与网速针对当前文件
        if filename and filename != self._current_filename:
            self._current_filename = filename
            self._last_bytes = 0
            self._last_ts = None
            job.downloaded_bytes = 0
            job.total_bytes = None
            job.current_file = filename

        # 进度条用：每个 chunk 都更新（保持平滑）
        job.downloaded_bytes = int(downloaded)
        if total is not None:
            job.total_bytes = int(total)

        # 网速用：按时间窗口采样，避免逐 chunk 计算的虚高尖峰
        now = time.monotonic()
        if self._last_ts is None:
            self._last_ts = now
            self._last_bytes = downloaded
            return
        dt = now - self._last_ts
        if dt < self._SPEED_SAMPLE_INTERVAL:
            return  # 窗口内只更新字节、不重算速度
        delta = downloaded - self._last_bytes
        inst = delta / dt if dt > 0 else 0.0
        # 指数移动平均平滑（alpha=0.4）；首采直接取瞬时值
        if job.speed_bps > 0:
            job.speed_bps = int(0.4 * inst + 0.6 * job.speed_bps)
        else:
            job.speed_bps = int(inst)
        self._last_ts = now
        self._last_bytes = downloaded
