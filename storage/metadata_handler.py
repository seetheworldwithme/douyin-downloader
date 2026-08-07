import asyncio
import json
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Optional

import aiofiles

from utils.logger import setup_logger

logger = setup_logger("MetadataHandler")


class MetadataHandler:
    def __init__(self):
        # 延迟到首次 append_download_manifest 在当前 event loop 上创建 Lock，
        # 与 Database._get_conn 一致，避免在 __init__ 阶段（可能处于错误的 loop）
        # 抢绑事件循环。
        self._manifest_lock: Optional[asyncio.Lock] = None

    async def save_metadata(self, data: Dict[str, Any], save_path: Path) -> bool:
        try:
            async with aiofiles.open(save_path, "w", encoding="utf-8") as f:
                await f.write(json.dumps(data, ensure_ascii=False, indent=2))
            return True
        except Exception as e:
            logger.error("Failed to save metadata: %s, error: %s", save_path, e)
            return False

    async def append_download_manifest(self, base_path: Path, record: Dict[str, Any]) -> bool:
        manifest_path = base_path / "download_manifest.jsonl"
        normalized_record = {
            "recorded_at": datetime.now().isoformat(timespec="seconds"),
            **record,
        }

        try:
            if self._manifest_lock is None:
                self._manifest_lock = asyncio.Lock()
            async with self._manifest_lock:
                async with aiofiles.open(manifest_path, "a", encoding="utf-8") as f:
                    await f.write(json.dumps(normalized_record, ensure_ascii=False))
                    await f.write("\n")
            return True
        except Exception as e:
            logger.error("Failed to append download manifest: %s, error: %s", manifest_path, e)
            return False
