import { registerPlugin } from '@capacitor/core'

// 原生插件 SaveToGalleryPlugin(Kotlin,@CapacitorPlugin name="SaveToGallery")。
// 仅在 Android(APK)里由 SubmitCard 调用:OkHttp 拉服务端 /stream → 写入
// MediaStore.Video(Movies/抖音下载器),视频进系统相册。
//
// 网页端永不调用(SubmitCard 先用 Capacitor.getPlatform() 判平台,web 上直接走
// <a download>),故这里不提供 web 实现 —— registerPlugin 在 web 上返回的代理
// 一旦被调用会抛 "not implemented",正是我们想要的兜底信号。
export const SaveToGallery = registerPlugin('SaveToGallery', {
  web: {
    // 占位:web 上不应走到这里。抛错便于发现误调用。
    async saveToGallery() {
      throw new Error('SaveToGallery 仅在 Android APP 内可用')
    },
  },
})
