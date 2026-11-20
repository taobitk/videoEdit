#pragma once

#include <condition_variable>
#include <functional>
#include <mutex>
#include <queue>
#include <thread>
#include <vector>

extern "C" {
#include <libavcodec/avcodec.h>
#include <libavutil/frame.h>
#include <libavutil/hwcontext.h>
}

class AsyncAv1Encoder {
public:
    static constexpr size_t kMaxInFlight = 16;

    ~AsyncAv1Encoder() {
        stopAndFree();
    }

    bool start(
        AVCodecContext* encCtx,
        std::function<void(AVPacket*)> onPacket,
        std::function<void(int64_t pts)> onEncodedFrame)
    {
        encCtx_ = encCtx;
        onPacket_ = std::move(onPacket);
        onEncodedFrame_ = std::move(onEncodedFrame);
        encPkt_ = av_packet_alloc();
        if (!encPkt_) return false;
        for (size_t i = 0; i < kMaxInFlight; ++i) {
            AVFrame* f = av_frame_alloc();
            if (!f) return false;
            freeQ_.push(f);
        }
        started_ = true;
        worker_ = std::thread(&AsyncAv1Encoder::run, this);
        return true;
    }

    bool submitGpuFrame(AVFrame* gpuFrame) {
        AVFrame* sw = nullptr;
        {
            std::unique_lock<std::mutex> lk(mu_);
            cvFree_.wait(lk, [&]() { return !freeQ_.empty() || stop_; });
            if (stop_) return false;
            sw = freeQ_.front();
            freeQ_.pop();
        }
        if (av_hwframe_transfer_data(sw, gpuFrame, 0) < 0 ||
            av_frame_copy_props(sw, gpuFrame) < 0) {
            av_frame_unref(sw);
            std::lock_guard<std::mutex> lk(mu_);
            freeQ_.push(sw);
            cvFree_.notify_one();
            return false;
        }
        {
            std::lock_guard<std::mutex> lk(mu_);
            workQ_.push(sw);
        }
        cvWork_.notify_one();
        return true;
    }

    void finish() {
        if (!started_) return;
        {
            std::lock_guard<std::mutex> lk(mu_);
            eof_ = true;
        }
        cvWork_.notify_all();
        if (worker_.joinable()) worker_.join();
        started_ = false;
    }

private:
    void run() {
        while (true) {
            AVFrame* sw = nullptr;
            {
                std::unique_lock<std::mutex> lk(mu_);
                cvWork_.wait(lk, [&]() { return !workQ_.empty() || eof_ || stop_; });
                if (workQ_.empty()) {
                    if (eof_ || stop_) break;
                    continue;
                }
                sw = workQ_.front();
                workQ_.pop();
            }
            int64_t pts = sw->pts;
            if (avcodec_send_frame(encCtx_, sw) >= 0) {
                while (avcodec_receive_packet(encCtx_, encPkt_) == 0) {
                    onPacket_(encPkt_);
                }
            }
            onEncodedFrame_(pts);
            av_frame_unref(sw);
            {
                std::lock_guard<std::mutex> lk(mu_);
                freeQ_.push(sw);
            }
            cvFree_.notify_one();
        }
        if (!stop_) {
            avcodec_send_frame(encCtx_, nullptr);
            while (avcodec_receive_packet(encCtx_, encPkt_) == 0) {
                onPacket_(encPkt_);
            }
        }
    }

    void stopAndFree() {
        if (started_) {
            {
                std::lock_guard<std::mutex> lk(mu_);
                stop_ = true;
            }
            cvWork_.notify_all();
            cvFree_.notify_all();
            if (worker_.joinable()) worker_.join();
            started_ = false;
        }
        while (!workQ_.empty()) {
            AVFrame* f = workQ_.front(); workQ_.pop(); av_frame_free(&f);
        }
        while (!freeQ_.empty()) {
            AVFrame* f = freeQ_.front(); freeQ_.pop(); av_frame_free(&f);
        }
        if (encPkt_) av_packet_free(&encPkt_);
    }

    AVCodecContext* encCtx_ = nullptr;
    AVPacket* encPkt_ = nullptr;
    std::function<void(AVPacket*)> onPacket_;
    std::function<void(int64_t)> onEncodedFrame_;
    std::mutex mu_;
    std::condition_variable cvWork_, cvFree_;
    std::queue<AVFrame*> workQ_, freeQ_;
    std::thread worker_;
    bool started_ = false, eof_ = false, stop_ = false;
};
