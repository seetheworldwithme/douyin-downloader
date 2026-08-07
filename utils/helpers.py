from datetime import datetime
from typing import Union


def parse_timestamp(timestamp: Union[int, str], fmt: str = "%Y-%m-%d %H:%M:%S") -> str:
    if isinstance(timestamp, str):
        timestamp = int(timestamp)
    return datetime.fromtimestamp(timestamp).strftime(fmt)


def format_size(bytes_size: int) -> str:
    for unit in ["B", "KB", "MB", "GB"]:
        if bytes_size < 1024.0:
            return f"{bytes_size:.2f} {unit}"
        bytes_size /= 1024.0
    return f"{bytes_size:.2f} TB"


