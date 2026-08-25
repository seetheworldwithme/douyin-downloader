package io.github.nick.dydl.util

import org.junit.Assert.assertEquals
import org.junit.Test

class UrlExtractorTest {

    @Test
    fun `完整分享文案提取短链`() {
        val text = "1.53 复制打开抖音,看看【某某的作品】超好看的一个视频 https://v.douyin.com/iABcDef/ 复制此链接,"
        assertEquals("https://v.douyin.com/iABcDef", UrlExtractor.extract(text))
    }

    @Test
    fun `www 视频链接`() {
        // query 参数原样保留(服务端 /resolve 自行解析,与网页版行为一致)
        assertEquals(
            "https://www.douyin.com/video/7301234567890123456?from=share",
            UrlExtractor.extract("看这个 https://www.douyin.com/video/7301234567890123456?from=share"),
        )
    }

    @Test
    fun `裸短链补协议`() {
        assertEquals(
            "https://v.douyin.com/iABcDef",
            UrlExtractor.extract("v.douyin.com/iABcDef/"),
        )
    }

    @Test
    fun `尾部标点去除`() {
        assertEquals(
            "https://v.douyin.com/iABcDef",
            UrlExtractor.extract("链接:https://v.douyin.com/iABcDef。。。!!,"),
        )
    }

    @Test
    fun `中文顿号右括号截断`() {
        // 分享文案常见 "https://v.douyin.com/xxx/）" 形式,链接取到 ) 之前
        assertEquals(
            "https://v.douyin.com/xyz123",
            UrlExtractor.extract("【抖音】https://v.douyin.com/xyz123/）好物推荐"),
        )
    }

    @Test
    fun `无链接返回空`() {
        assertEquals("", UrlExtractor.extract("今天天气不错"))
        assertEquals("", UrlExtractor.extract(""))
    }

    @Test
    fun `多链接取第一个`() {
        val text = "https://v.douyin.com/first123/ 和 https://v.douyin.com/second456/"
        assertEquals("https://v.douyin.com/first123", UrlExtractor.extract(text))
    }
}
