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
 * SaveToGallery —— 把服务端流式文件写入系统媒体库。
 *
 * 由前端 web/src/plugins/saveToGallery.js 通过 registerPlugin('SaveToGallery') 调用。
 * 进度经 notifyListeners("saveProgress", {percent}) 推回 JS。
 *
 * 参数:
 *   url      - 服务端 /stream 地址(必须)
 *   filename - 保存的文件名(含扩展名)
 *   mime     - MIME 类型,默认 video/mp4
 *   album    - 保存位置:movies(默认,相册)/ pictures(相册)/ downloads(下载目录)
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

    /** 保存目标:MediaStore collection + 相对目录。 */
    private static class Target {
        final Uri collection;
        final String relativePath;

        Target(Uri collection, String relativePath) {
            this.collection = collection;
            this.relativePath = relativePath;
        }
    }

    private Target targetFor(String album) {
        String volume = MediaStore.VOLUME_EXTERNAL_PRIMARY;
        if ("pictures".equals(album)) {
            return new Target(
                    MediaStore.Images.Media.getContentUri(volume),
                    "Pictures/" + FOLDER);
        }
        if ("downloads".equals(album)) {
            return new Target(
                    MediaStore.Downloads.getContentUri(volume),
                    "Download/" + FOLDER);
        }
        return new Target(
                MediaStore.Video.Media.getContentUri(volume),
                "Movies/" + FOLDER);
    }

    @PluginMethod
    public void saveToGallery(PluginCall call) {
        String url = call.getString("url");
        if (url == null || url.trim().isEmpty()) {
            call.reject("url is required");
            return;
        }
        String mime = call.getString("mime");
        if (mime == null || mime.trim().isEmpty()) {
            mime = "video/mp4";
        }
        final String contentType = mime;
        final Target target = targetFor(call.getString("album"));
        String rawName = call.getString("filename");
        final String filename = sanitize(rawName != null ? rawName : "", contentType);
        final String downloadUrl = url;

        new Thread(() -> {
            try {
                OkHttpClient client = new OkHttpClient.Builder()
                        .connectTimeout(30, TimeUnit.SECONDS)
                        // 图集"合成视频"在服务端要跑 ffmpeg,首字节可能几十秒后才到,
                        // 所以读超时给足(它是两次数据包之间的间隔,不是总时长)。
                        .readTimeout(300, TimeUnit.SECONDS)
                        .build();
                Response response = client.newCall(new Request.Builder().url(downloadUrl).build()).execute();
                if (!response.isSuccessful()) {
                    String detail = "上游返回 " + response.code();
                    ResponseBody body = response.body();
                    if (body != null) {
                        try {
                            String text = body.string();
                            if (text != null && text.length() > 400) text = text.substring(0, 400);
                            if (text != null && text.contains("detail")) detail += ": " + text;
                        } catch (Exception ignore) {
                        }
                    }
                    call.reject(detail);
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
                values.put(MediaStore.MediaColumns.DISPLAY_NAME, filename);
                values.put(MediaStore.MediaColumns.MIME_TYPE, contentType);
                values.put(MediaStore.MediaColumns.RELATIVE_PATH, target.relativePath);
                // API 29+ 起支持 IS_PENDING:写入期间对其它应用不可见,写完清零
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    values.put(MediaStore.MediaColumns.IS_PENDING, 1);
                }
                Uri uri = resolver.insert(target.collection, values);
                if (uri == null) {
                    call.reject("无法创建媒体记录(存储卷可能不可用)");
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

                    // 写完:清除 IS_PENDING,媒体库立即可见
                    ContentValues done = new ContentValues();
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                        done.put(MediaStore.MediaColumns.IS_PENDING, 0);
                    }
                    resolver.update(uri, done, null, null);

                    JSObject ret = new JSObject();
                    ret.put("uri", uri.toString());
                    call.resolve(ret);
                } catch (Exception e) {
                    // 失败时删除半成品记录,避免媒体库残留损坏项
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

    /** 去掉文件名非法字符;没有扩展名时按 MIME 补一个。 */
    private String sanitize(String name, String mime) {
        String safe = name.replaceAll("[\\\\/:*?\"<>|]", "_").trim();
        if (safe.isEmpty()) {
            safe = "douyin";
        }
        if (!safe.contains(".")) {
            String ext = ".bin";
            if (mime.startsWith("video/")) ext = mime.contains("webm") ? ".webm" : ".mp4";
            else if (mime.equals("image/jpeg")) ext = ".jpg";
            else if (mime.startsWith("image/")) ext = "." + mime.substring("image/".length());
            else if (mime.equals("application/zip")) ext = ".zip";
            safe = safe + ext;
        }
        return safe;
    }
}
