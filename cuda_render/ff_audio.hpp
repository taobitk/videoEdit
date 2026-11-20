#pragma once

#include <algorithm>
#include <cmath>
#include <cstdio>
#include <string>

extern "C" {
#include <libavcodec/avcodec.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavformat/avformat.h>
#include <libavutil/channel_layout.h>
#include <libavutil/opt.h>
}

inline bool canStreamCopyAudio(AVStream* aStream, double speed, bool hasTrim) {
    if (!aStream || hasTrim || std::abs(speed - 1.0) > 0.001) return false;
    AVCodecID id = aStream->codecpar->codec_id;
    return id == AV_CODEC_ID_AAC || id == AV_CODEC_ID_MP3;
}

class AudioTranscoder {
public:
    ~AudioTranscoder() { cleanup(); }

    bool init(AVStream* inAStream, AVFormatContext* outFmt, const std::string& atempoExpr, std::string& errOut) {
        inStream_ = inAStream;
        outFmt_ = outFmt;
        const AVCodec* dec = avcodec_find_decoder(inAStream->codecpar->codec_id);
        if (!dec) { errOut = "Audio decoder not found"; return false; }
        decCtx_ = avcodec_alloc_context3(dec);
        avcodec_parameters_to_context(decCtx_, inAStream->codecpar);
        if (avcodec_open2(decCtx_, dec, nullptr) < 0) {
            errOut = "Failed to open audio decoder";
            return false;
        }

        const AVCodec* enc = avcodec_find_encoder(AV_CODEC_ID_AAC);
        if (!enc) { errOut = "AAC encoder not found"; return false; }
        encCtx_ = avcodec_alloc_context3(enc);
        encCtx_->sample_rate = 48000;
        av_channel_layout_default(&encCtx_->ch_layout, 2);
        encCtx_->sample_fmt = AV_SAMPLE_FMT_FLTP;
        encCtx_->bit_rate = 128000;
        encCtx_->time_base = AVRational{1, 48000};
        if (outFmt->oformat->flags & AVFMT_GLOBALHEADER) encCtx_->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;
        if (avcodec_open2(encCtx_, enc, nullptr) < 0) {
            errOut = "Failed to open AAC encoder";
            return false;
        }

        outStream_ = avformat_new_stream(outFmt, nullptr);
        avcodec_parameters_from_context(outStream_->codecpar, encCtx_);
        outStream_->time_base = encCtx_->time_base;

        fg_ = avfilter_graph_alloc();
        char chBuf[64];
        av_channel_layout_describe(&decCtx_->ch_layout, chBuf, sizeof(chBuf));
        char args[256];
        std::snprintf(args, sizeof(args),
            "time_base=%d/%d:sample_rate=%d:sample_fmt=%s:channel_layout=%s",
            inStream_->time_base.num, inStream_->time_base.den,
            decCtx_->sample_rate, av_get_sample_fmt_name(decCtx_->sample_fmt), chBuf);
        if (avfilter_graph_create_filter(&bufSrc_, avfilter_get_by_name("abuffer"), "in", args, nullptr, fg_) < 0 ||
            avfilter_graph_create_filter(&bufSink_, avfilter_get_by_name("abuffersink"), "out", nullptr, nullptr, fg_) < 0) {
            errOut = "Audio filter buffer creation failed";
            return false;
        }

        std::string desc = atempoExpr.empty()
            ? "aformat=sample_fmts=fltp:channel_layouts=stereo:sample_rates=48000,asetnsamples=n=1024:p=0"
            : (atempoExpr + ",aformat=sample_fmts=fltp:channel_layouts=stereo:sample_rates=48000,asetnsamples=n=1024:p=0");
        AVFilterInOut *outputs = avfilter_inout_alloc(), *inputs = avfilter_inout_alloc();
        outputs->name = av_strdup("in");  outputs->filter_ctx = bufSrc_;  outputs->pad_idx = 0; outputs->next = nullptr;
        inputs->name  = av_strdup("out"); inputs->filter_ctx  = bufSink_; inputs->pad_idx  = 0; inputs->next  = nullptr;
        bool ok = (avfilter_graph_parse_ptr(fg_, desc.c_str(), &inputs, &outputs, nullptr) >= 0 &&
                   avfilter_graph_config(fg_, nullptr) >= 0);
        avfilter_inout_free(&inputs);
        avfilter_inout_free(&outputs);
        if (!ok) {
            errOut = "Audio filtergraph config failed: " + desc;
            return false;
        }
        inFrm_ = av_frame_alloc();
        filtFrm_ = av_frame_alloc();
        encPkt_ = av_packet_alloc();
        return true;
    }

    void resetDecoderForSeek() {
        if (decCtx_) avcodec_flush_buffers(decCtx_);
    }

    bool feedPacket(AVPacket* pkt, double segStart, double segEnd, double keptOffset, bool headerWritten) {
        if (!decCtx_ || !headerWritten) return true;
        if (avcodec_send_packet(decCtx_, pkt) < 0) return true;
        while (avcodec_receive_frame(decCtx_, inFrm_) == 0) {
            processDecodedFrame(inFrm_, segStart, segEnd, keptOffset);
            av_frame_unref(inFrm_);
        }
        return true;
    }

    void flush(bool headerWritten) {
        if (!decCtx_ || !headerWritten) return;
        avcodec_send_packet(decCtx_, nullptr);
        while (avcodec_receive_frame(decCtx_, inFrm_) == 0) {
            av_frame_unref(inFrm_);
        }
        if (av_buffersrc_add_frame_flags(bufSrc_, nullptr, 0) >= 0) {
            drainFilterAndEncode();
        }
        avcodec_send_frame(encCtx_, nullptr);
        drainEncoderPackets();
    }

private:
    void processDecodedFrame(AVFrame* f, double segStart, double segEnd, double keptOffset) {
        double tb = av_q2d(inStream_->time_base);
        double t = (f->pts != AV_NOPTS_VALUE) ? f->pts * tb : 0.0;
        if (t < segStart - 0.02 || t >= segEnd) return;

        double rel = std::max(0.0, t - segStart);
        int64_t newPts = static_cast<int64_t>(std::llround((keptOffset + rel) / tb));
        if (lastInPts_ != AV_NOPTS_VALUE && newPts <= lastInPts_) {
            newPts = lastInPts_ + std::max<int64_t>(1, static_cast<int64_t>(std::llround((static_cast<double>(f->nb_samples) / f->sample_rate) / tb)));
        }
        lastInPts_ = newPts;
        f->pts = newPts;
        if (av_buffersrc_add_frame_flags(bufSrc_, f, 0) >= 0) {
            drainFilterAndEncode();
        }
    }

    void drainFilterAndEncode() {
        while (av_buffersink_get_frame(bufSink_, filtFrm_) >= 0) {
            if (avcodec_send_frame(encCtx_, filtFrm_) >= 0) {
                drainEncoderPackets();
            }
            av_frame_unref(filtFrm_);
        }
    }

    void drainEncoderPackets() {
        while (avcodec_receive_packet(encCtx_, encPkt_) == 0) {
            av_packet_rescale_ts(encPkt_, encCtx_->time_base, outStream_->time_base);
            encPkt_->stream_index = outStream_->index;
            av_interleaved_write_frame(outFmt_, encPkt_);
            av_packet_unref(encPkt_);
        }
    }

    void cleanup() {
        if (encPkt_) av_packet_free(&encPkt_);
        if (inFrm_) av_frame_free(&inFrm_);
        if (filtFrm_) av_frame_free(&filtFrm_);
        if (fg_) avfilter_graph_free(&fg_);
        if (encCtx_) avcodec_free_context(&encCtx_);
        if (decCtx_) avcodec_free_context(&decCtx_);
    }

    AVStream* inStream_ = nullptr;
    AVStream* outStream_ = nullptr;
    AVFormatContext* outFmt_ = nullptr;
    AVCodecContext* decCtx_ = nullptr;
    AVCodecContext* encCtx_ = nullptr;
    AVFilterGraph* fg_ = nullptr;
    AVFilterContext *bufSrc_ = nullptr, *bufSink_ = nullptr;
    AVFrame *inFrm_ = nullptr, *filtFrm_ = nullptr;
    AVPacket* encPkt_ = nullptr;
    int64_t lastInPts_ = AV_NOPTS_VALUE;
};
