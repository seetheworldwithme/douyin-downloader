package io.github.nick.dydl;

import android.content.ContentResolver;
import android.content.ContentValues;
import android.net.Uri;
import android.os.Build;
import android.provider.MediaStore;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;

import java.io.InputStream;
import java.io.OutputStream;
import java.util.concurrent.TimeUnit;

/**
 * SaveToGallery —— 把服务端流式 mp4 直接写入系统相册(Movies/抖音下载器)。
 *
 * 由前端 web/src/plugins/saveToGallery.js 通过 registerPlugin('SaveToGallery') 调用。
 * 进度经 notifyListeners("saveProgress", {percent}) 推回 JS。
 *
 * 放置位置:`cap add android` 后,复制到
 *   web/android/app/src/main/java/io/github/nick/dydl/SaveToGalleryPlugin.java
 * 并在 MainActivity.onCreate 里 registerPlugin(SaveToGalleryPlugin.class)。
 *
 * 用 Java(而非 Kotlin):Capacitor 默认安卓模板不带 Kotlin 插件,纯 Java 最省事。
 * 仅支持 API 29+(minSdk 29):走 MediaStore 分区存储,无需运行时存储权限。
 */
@CapacitorPlugin(name = "SaveToGallery")
public class SaveToGalleryPlugin extends Plugin {

    private static final String FOLDER = "抖音下载器";
    private static final int BUF_SIZE = 256 * 1024;

    private Uri videoCollection() {
        return MediaStore.Video.Media.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY);
    }

    @PluginMethod
    public void saveToGallery(PluginCall call) {
        String url = call.getString("url");
        if (url == null || url.trim().isEmpty()) {
            call.reject("url is required");
            return;
        }
        String rawName = call.getString("filename");
        final String filename = sanitize(rawName != null ? rawName : "video.mp4");
        final String downloadUrl = url;

        new Thread(() -> {
            try {
                OkHttpClient client = new OkHttpClient.Builder()
                        .connectTimeout(30, TimeUnit.SECONDS)
                        .readTimeout(60, TimeUnit.SECONDS)
                        .build();
                Response response = client.newCall(new Request.Builder().url(downloadUrl).build()).execute();
                if (!response.isSuccessful()) {
                    call.reject("上游返回 " + response.code());
                    return;
                }
                ResponseBody body = response.body();
                if (body == null) {
                    call.reject("空响应体");
                    return;
                }
                long total = body.contentLength();

                ContentResolver resolver = getContext().getContentResolver();
                ContentValues values = new ContentValues();
                values.put(MediaStore.Video.Media.DISPLAY_NAME, filename);
                values.put(MediaStore.Video.Media.MIME_TYPE, "video/mp4");
                values.put(MediaStore.Video.Media.RELATIVE_PATH, "Movies/" + FOLDER);
                // API 29+ 起支持 IS_PENDING:写入期间对其它应用不可见,写完清零
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    values.put(MediaStore.Video.Media.IS_PENDING, 1);
                }
                Uri uri = resolver.insert(videoCollection(), values);
                if (uri == null) {
                    call.reject("无法创建媒体记录(Movies 卷可能不可用)");
                    return;
                }
                OutputStream out = resolver.openOutputStream(uri, "w");
                if (out == null) {
                    resolver.delete(uri, null, null);
                    call.reject("无法打开输出流");
                    return;
                }
                try {
                    InputStream input = body.byteStream();
                    byte[] buf = new byte[BUF_SIZE];
                    long read = 0L;
                    int pct = -1;
                    int n;
                    while ((n = input.read(buf)) != -1) {
                        out.write(buf, 0, n);
                        read += n;
                        if (total > 0) {
                            int p = (int) (read * 100 / total);
                            if (p != pct) {
                                pct = p;
                                notifyProgress(pct);
                            }
                        }
                    }
                    out.flush();
                    input.close();
                    out.close();

                    // 写完:清除 IS_PENDING,相册立即可见
                    ContentValues done = new ContentValues();
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                        done.put(MediaStore.Video.Media.IS_PENDING, 0);
                    }
                    resolver.update(uri, done, null, null);

                    JSObject ret = new JSObject();
                    ret.put("uri", uri.toString());
                    call.resolve(ret);
                } catch (Exception e) {
                    // 失败时删除半成品记录,避免相册残留损坏项
                    try {
                        resolver.delete(uri, null, null);
                    } catch (Exception ignore) {
                    }
                    call.reject(e.getLocalizedMessage() != null ? e.getLocalizedMessage() : "写入失败", e);
                }
            } catch (Exception e) {
                call.reject(e.getLocalizedMessage() != null ? e.getLocalizedMessage() : "下载失败", e);
            }
        }).start();
    }

    private void notifyProgress(int percent) {
        JSObject obj = new JSObject();
        obj.put("percent", percent);
        notifyListeners("saveProgress", obj);
    }

    /** 去掉文件名非法字符,保证以 .mp4 结尾。 */
    private String sanitize(String name) {
        String safe = name.replaceAll("[\\\\/:*?\"<>|]", "_").trim();
        if (safe.isEmpty()) {
            safe = "video";
        }
        return safe.toLowerCase().endsWith(".mp4") ? safe : safe + ".mp4";
    }
}
