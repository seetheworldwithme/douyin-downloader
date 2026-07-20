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

    def __init__(self, job: DownloadJob):
        self._job = job
        # 指数移动平均的网速计算状态
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

        job.downloaded_bytes = int(downloaded)
        if total is not None:
            job.total_bytes = int(total)

        now = time.monotonic()
        if self._last_ts is not None:
            dt = now - self._last_ts
            if dt > 0:
                inst = (downloaded - self._last_bytes) / dt
                # 指数移动平均，平滑抖动（alpha=0.3）
                job.speed_bps = int(0.3 * inst + 0.7 * job.speed_bps)
        self._last_ts = now
        self._last_bytes = downloaded
