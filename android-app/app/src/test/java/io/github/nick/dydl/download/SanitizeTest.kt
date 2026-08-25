package io.github.nick.dydl.download

import org.junit.Assert.assertEquals
import org.junit.Test

class SanitizeTest {

    @Test
    fun `非法字符替换为下划线`() {
        // 无扩展名会按 mime 补 .mp4(与原 Java 插件行为一致)
        assertEquals("a_b_c.mp4", MediaStoreDownloader.sanitize("a/b\\c", "video/mp4"))
        assertEquals("视频_名字_.mp4", MediaStoreDownloader.sanitize("视频:名字?.mp4", "video/mp4"))
    }

    @Test
    fun `空名回退并按 mime 补扩展名`() {
        assertEquals("douyin.mp4", MediaStoreDownloader.sanitize("", "video/mp4"))
        assertEquals("douyin.jpg", MediaStoreDownloader.sanitize("", "image/jpeg"))
        assertEquals("douyin.zip", MediaStoreDownloader.sanitize("", "application/zip"))
        assertEquals("douyin.png", MediaStoreDownloader.sanitize("", "image/png"))
        assertEquals("douyin.bin", MediaStoreDownloader.sanitize("  ", "text/plain"))
    }

    @Test
    fun `无扩展名按 mime 补`() {
        assertEquals("abc.jpg", MediaStoreDownloader.sanitize("abc", "image/jpeg"))
        assertEquals("abc.mp4", MediaStoreDownloader.sanitize("abc", "video/mp4"))
    }

    @Test
    fun `已有扩展名不重复补`() {
        assertEquals("abc.png", MediaStoreDownloader.sanitize("abc.png", "video/mp4"))
    }

    @Test
    fun `webm 扩展名`() {
        assertEquals("x.webm", MediaStoreDownloader.sanitize("x", "video/webm"))
    }
}
