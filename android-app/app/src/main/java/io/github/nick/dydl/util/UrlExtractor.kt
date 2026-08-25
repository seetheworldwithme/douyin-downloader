package io.github.nick.dydl.util

/**
 * 从粘贴文本中提取抖音链接 —— 移植自 web/src/components/SubmitCard.vue 的 extractUrl。
 *
 * 粘贴的可能是整段分享文案(含文案 + 短链 + 尾部乱码),从中提取第一个抖音链接。
 */
object UrlExtractor {

    /** 1) 带协议的链接(覆盖 https://v.douyin.com/xxx、www.douyin.com/video/xxx 等) */
    private val WITH_SCHEME = Regex("""https?://[^\s，。、,）)】]+""")

    /** 2) 兜底:不带协议的裸短链 v.douyin.com/xxxxx */
    private val BARE_SHORT = Regex("""\bv\.douyin\.com/[A-Za-z0-9_-]+""")

    /** 去掉链接尾部的标点和斜杠 */
    private fun normalize(u: String): String =
        u.trimEnd('/').replace(Regex("""[.,，。!！?？]+$"""), "").trimEnd('/')

    /** 返回提取到的链接;识别不到返回空串 */
    fun extract(text: String): String {
        if (text.isBlank()) return ""
        WITH_SCHEME.find(text)?.let { return normalize(it.value) }
        BARE_SHORT.find(text)?.let { return normalize("https://" + it.value) }
        return ""
    }
}
