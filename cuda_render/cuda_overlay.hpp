#pragma once

#include <cstdint>
#include <cstdio>
#include <cstring>
#include <string>
#include <vector>

extern "C" {
#include <libavcodec/avcodec.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
}

struct CudaOverlay {
    std::string path;
    int targetW = 0;
    double xRatio = 0.5;
    double yRatio = 0.5;
    double opacity = 1.0;
    double startSec = 0.0;
    double endSec = 1e12;
    int w = 0;
    int h = 0;
    unsigned long long dPtr = 0;
};

inline bool loadOverlayRGBA(
    const std::string& path,
    int targetW,
    std::vector<uint8_t>& rgbaOut,
    int& wOut,
    int& hOut,
    std::string& errOut)
{
    AVFormatContext* fmt = nullptr;
    if (avformat_open_input(&fmt, path.c_str(), nullptr, nullptr) < 0 ||
        avformat_find_stream_info(fmt, nullptr) < 0) {
        if (fmt) avformat_close_input(&fmt);
        errOut = "Failed to open overlay image: " + path;
        return false;
    }
    int sIdx = av_find_best_stream(fmt, AVMEDIA_TYPE_VIDEO, -1, -1, nullptr, 0);
    if (sIdx < 0) {
        avformat_close_input(&fmt);
        errOut = "No image stream in overlay: " + path;
        return false;
    }
    const AVCodec* dec = avcodec_find_decoder(fmt->streams[sIdx]->codecpar->codec_id);
    AVCodecContext* decCtx = avcodec_alloc_context3(dec);
    avcodec_parameters_to_context(decCtx, fmt->streams[sIdx]->codecpar);
    if (avcodec_open2(decCtx, dec, nullptr) < 0) {
        avcodec_free_context(&decCtx);
        avformat_close_input(&fmt);
        errOut = "Failed to open overlay decoder: " + path;
        return false;
    }

    AVPacket* pkt = av_packet_alloc();
    AVFrame* frm = av_frame_alloc();
    bool gotFrame = false;
    while (av_read_frame(fmt, pkt) >= 0) {
        if (pkt->stream_index == sIdx && avcodec_send_packet(decCtx, pkt) >= 0) {
            if (avcodec_receive_frame(decCtx, frm) == 0) {
                gotFrame = true;
                av_packet_unref(pkt);
                break;
            }
        }
        av_packet_unref(pkt);
    }
    if (!gotFrame) {
        avcodec_send_packet(decCtx, nullptr);
        gotFrame = (avcodec_receive_frame(decCtx, frm) == 0);
    }
    av_packet_free(&pkt);
    avcodec_free_context(&decCtx);
    avformat_close_input(&fmt);
    if (!gotFrame) {
        av_frame_free(&frm);
        errOut = "Failed to decode overlay frame: " + path;
        return false;
    }

    AVFilterGraph* fg = avfilter_graph_alloc();
    AVFilterContext *src = nullptr, *sink = nullptr;
    char args[256];
    std::snprintf(args, sizeof(args),
        "video_size=%dx%d:pix_fmt=%d:time_base=1/25:pixel_aspect=1/1",
        frm->width, frm->height, frm->format);
    if (avfilter_graph_create_filter(&src, avfilter_get_by_name("buffer"), "in", args, nullptr, fg) < 0 ||
        avfilter_graph_create_filter(&sink, avfilter_get_by_name("buffersink"), "out", nullptr, nullptr, fg) < 0) {
        avfilter_graph_free(&fg);
        av_frame_free(&frm);
        errOut = "Overlay filter creation failed";
        return false;
    }

    char desc[128];
    if (targetW > 0) {
        std::snprintf(desc, sizeof(desc), "format=rgba,scale=%d:-1:flags=lanczos", targetW);
    } else {
        std::snprintf(desc, sizeof(desc), "format=rgba");
    }
    AVFilterInOut *outputs = avfilter_inout_alloc(), *inputs = avfilter_inout_alloc();
    outputs->name = av_strdup("in");  outputs->filter_ctx = src;  outputs->pad_idx = 0; outputs->next = nullptr;
    inputs->name  = av_strdup("out"); inputs->filter_ctx  = sink; inputs->pad_idx  = 0; inputs->next  = nullptr;
    bool ok = (avfilter_graph_parse_ptr(fg, desc, &inputs, &outputs, nullptr) >= 0 &&
               avfilter_graph_config(fg, nullptr) >= 0);
    avfilter_inout_free(&inputs);
    avfilter_inout_free(&outputs);

    AVFrame* rgbaFrm = av_frame_alloc();
    if (ok && av_buffersrc_add_frame(src, frm) >= 0 && av_buffersink_get_frame(sink, rgbaFrm) >= 0) {
        wOut = rgbaFrm->width;
        hOut = rgbaFrm->height;
        rgbaOut.resize(static_cast<size_t>(wOut) * hOut * 4);
        for (int y = 0; y < hOut; ++y) {
            std::memcpy(rgbaOut.data() + static_cast<size_t>(y) * wOut * 4,
                        rgbaFrm->data[0] + y * rgbaFrm->linesize[0],
                        static_cast<size_t>(wOut) * 4);
        }
    } else {
        ok = false;
        errOut = "Overlay scale/format=rgba failed";
    }
    av_frame_free(&rgbaFrm);
    av_frame_free(&frm);
    avfilter_graph_free(&fg);
    return ok;
}
