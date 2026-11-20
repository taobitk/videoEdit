import { expect, test, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

interface BBox {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
  width: number;
  height: number;
  cx: number;
  cy: number;
  litPixels: number;
}

interface ParityResult {
  label: string;
  preview: BBox;
  render: BBox;
  dcx: number;
  dcy: number;
  dw: number;
  dh: number;
  iou: number;
}

let tempDir = '';
let black16x9 = '';
let black9x16 = '';
let logoPng = '';
const createdDownloads: string[] = [];

function findFfmpeg(): string {
  const wingetPath =
    'C:\\Users\\8hung\\AppData\\Local\\Microsoft\\WinGet\\Packages\\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\\ffmpeg-8.1.1-full_build\\bin\\ffmpeg.exe';
  return fs.existsSync(wingetPath) ? wingetPath : 'ffmpeg';
}

function measureBBox(rgba: Uint8Array | number[], w: number, h: number, threshold = 45): BBox {
  let minX = w;
  let minY = h;
  let maxX = -1;
  let maxY = -1;
  let litPixels = 0;

  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const idx = (y * w + x) * 4;
      const a = rgba[idx + 3];
      if (a < 30) continue;
      const lum = Math.max(rgba[idx], rgba[idx + 1], rgba[idx + 2]) * (a / 255);
      if (lum >= threshold) {
        litPixels++;
        if (x < minX) minX = x;
        if (x > maxX) maxX = x;
        if (y < minY) minY = y;
        if (y > maxY) maxY = y;
      }
    }
  }

  if (maxX < minX || maxY < minY) {
    return { minX: 0, minY: 0, maxX: 0, maxY: 0, width: 0, height: 0, cx: 0, cy: 0, litPixels: 0 };
  }
  const width = maxX - minX + 1;
  const height = maxY - minY + 1;
  return {
    minX,
    minY,
    maxX,
    maxY,
    width,
    height,
    cx: minX + width / 2,
    cy: minY + height / 2,
    litPixels
  };
}

function computeIoU(a: BBox, b: BBox): number {
  const ix1 = Math.max(a.minX, b.minX);
  const iy1 = Math.max(a.minY, b.minY);
  const ix2 = Math.min(a.maxX, b.maxX);
  const iy2 = Math.min(a.maxY, b.maxY);
  if (ix2 < ix1 || iy2 < iy1) return 0;
  const inter = (ix2 - ix1 + 1) * (iy2 - iy1 + 1);
  const union = a.width * a.height + b.width * b.height - inter;
  return union > 0 ? inter / union : 0;
}

async function captureKonvaRgba(page: Page, videoW: number, videoH: number): Promise<Uint8Array> {
  const result = await page.evaluate(({ w, h }) => {
    const container = document.querySelector('[data-testid="konva-stage-container"]');
    const srcCanvas = container?.querySelector('canvas');
    if (!srcCanvas) throw new Error('Konva canvas not found');
    const srcCtx = srcCanvas.getContext('2d');
    if (srcCanvas.width === w && srcCanvas.height === h && srcCtx) {
      return {
        canvasW: srcCanvas.width,
        canvasH: srcCanvas.height,
        data: Array.from(srcCtx.getImageData(0, 0, w, h).data)
      };
    }
    const off = document.createElement('canvas');
    off.width = w;
    off.height = h;
    const ctx = off.getContext('2d');
    if (!ctx) throw new Error('2D context unavailable');
    ctx.drawImage(srcCanvas, 0, 0, w, h);
    return {
      canvasW: srcCanvas.width,
      canvasH: srcCanvas.height,
      data: Array.from(ctx.getImageData(0, 0, w, h).data)
    };
  }, { w: videoW, h: videoH });
  expect(result.canvasW).toBe(videoW);
  expect(result.canvasH).toBe(videoH);
  return Uint8Array.from(result.data);
}

function extractRenderFrameRgba(videoPath: string, w: number, h: number): Uint8Array {
  const ffmpeg = findFfmpeg();
  const rawOut = path.join(tempDir, `frame_${Date.now()}.rgba`);
  execFileSync(ffmpeg, [
    '-y', '-v', 'error', '-i', videoPath, '-vframes', '1',
    '-f', 'rawvideo', '-pix_fmt', 'rgba', rawOut
  ]);
  const buf = fs.readFileSync(rawOut);
  fs.unlinkSync(rawOut);
  if (buf.length < w * h * 4) {
    throw new Error(`Unexpected raw frame size ${buf.length}, expected ${w * h * 4}`);
  }
  return new Uint8Array(buf.buffer, buf.byteOffset, w * h * 4);
}

async function renderAndMeasureCase(
  page: Page,
  label: string,
  videoPath: string,
  w: number,
  h: number,
  deselectCorner: 'tl' | 'br'
): Promise<ParityResult> {
  const stageBox = await page.getByTestId('konva-stage-container').boundingBox();
  if (!stageBox) throw new Error('Missing Konva stage boundingBox');
  const clickX = deselectCorner === 'tl' ? stageBox.x + 2 : stageBox.x + stageBox.width - 2;
  const clickY = deselectCorner === 'tl' ? stageBox.y + 2 : stageBox.y + stageBox.height - 2;
  await page.mouse.click(clickX, clickY);
  await page.waitForTimeout(200);

  let preview = measureBBox(await captureKonvaRgba(page, w, h), w, h, 45);
  for (let i = 0; i < 15 && preview.litPixels < 100; i++) {
    await page.waitForTimeout(100);
    preview = measureBBox(await captureKonvaRgba(page, w, h), w, h, 45);
  }

  await page.getByTestId('btn-start-process').click();
  await expect(page.getByTestId('render-completed-view')).toBeVisible({ timeout: 30_000 });

  const statusRes = await page.request.get('/api/status');
  const status = await statusRes.json();
  expect(String(status.encoder_detail || '')).toContain('CUDA NVRTC Zero-Copy+NVENC');
  expect(Number(status.fps || 0)).toBeGreaterThan(0);
  expect(String(status.speed || '0x')).not.toBe('0x');
  const outUrl = String(status.output_url || '');
  expect(outUrl).not.toBe('');

  const fileRes = await page.request.get(outUrl);
  expect(fileRes.ok()).toBeTruthy();
  const outPath = path.join(tempDir, `rendered_${Date.now()}.mp4`);
  fs.writeFileSync(outPath, await fileRes.body());
  createdDownloads.push(outPath);

  const renderRgba = extractRenderFrameRgba(outPath, w, h);
  const render = measureBBox(renderRgba, w, h, 45);

  const result: ParityResult = {
    label,
    preview,
    render,
    dcx: Math.round(Math.abs(preview.cx - render.cx) * 10) / 10,
    dcy: Math.round(Math.abs(preview.cy - render.cy) * 10) / 10,
    dw: Math.abs(preview.width - render.width),
    dh: Math.abs(preview.height - render.height),
    iou: Math.round(computeIoU(preview, render) * 1000) / 10
  };
  console.log(
    `[PIXEL PARITY] ${label} (${w}x${h}) -> Preview=[w:${preview.width}, h:${preview.height}, cx:${preview.cx.toFixed(1)}, cy:${preview.cy.toFixed(1)}] ` +
    `Render=[w:${render.width}, h:${render.height}, cx:${render.cx.toFixed(1)}, cy:${render.cy.toFixed(1)}] ` +
    `| Δcx=${result.dcx}px, Δcy=${result.dcy}px, Δw=${result.dw}px, Δh=${result.dh}px, IoU=${result.iou}%`
  );
  await page.getByTestId('btn-new-render').click();
  return result;
}

test.beforeAll(() => {
  tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 've-parity-'));
  const ffmpeg = findFfmpeg();
  black16x9 = path.join(tempDir, 'black_tiếng_việt_1280x720.mp4');
  black9x16 = path.join(tempDir, 'black_720x1280.mp4');
  logoPng = path.join(tempDir, 'logo_240x120.png');
  execFileSync(ffmpeg, [
    '-y', '-v', 'error', '-f', 'lavfi', '-i', 'color=c=black:s=1280x720:r=30:d=1',
    '-c:v', 'libx264', '-pix_fmt', 'yuv420p', black16x9
  ]);
  execFileSync(ffmpeg, [
    '-y', '-v', 'error', '-f', 'lavfi', '-i', 'color=c=black:s=720x1280:r=30:d=1',
    '-c:v', 'libx264', '-pix_fmt', 'yuv420p', black9x16
  ]);
  execFileSync(ffmpeg, [
    '-y', '-v', 'error', '-f', 'lavfi', '-i', 'color=c=0x00FFCC:s=240x120',
    '-vframes', '1', logoPng
  ]);
});

test.afterAll(() => {
  for (const p of createdDownloads) {
    if (fs.existsSync(p)) fs.rmSync(p, { force: true });
  }
  if (tempDir && fs.existsSync(tempDir)) {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('pixel-level parity between Konva preview and FFmpeg output (16:9 & 9:16)', async ({ page }) => {
  test.setTimeout(90_000);
  await page.goto('/');

  // Case 1: 1280x720 centered single-line bold text
  await page.getByTestId('input-edit-path').fill(black16x9);
  await page.getByTestId('btn-probe-edit').click();
  await expect(page.getByTestId('edit-probe-meta')).toBeVisible({ timeout: 10_000 });
  await page.getByTestId('btn-add-text-layer').click();
  await page.getByTestId('input-text-content').fill('VELOCITY 123');
  await page.getByTestId('btn-font-XL').click();
  await page.getByTestId('align-pt-mc').click();
  const c1 = await renderAndMeasureCase(page, '16:9 Single-Line Center', black16x9, 1280, 720, 'tl');

  // Case 2: 1280x720 top-left with bright cyan background box
  await page.getByTestId('layer-row-' + await page.locator('[data-testid^="layer-row-"]').first().getAttribute('data-testid').then((v) => v!.replace('layer-row-', ''))).click();
  await page.getByTestId('align-pt-tl').click();
  await page.getByTestId('toggle-text-bg').click();
  const bgColorInput = page.locator('input[type="color"]').nth(2);
  await bgColorInput.fill('#00ccff');
  await bgColorInput.dispatchEvent('input');
  const c2 = await renderAndMeasureCase(page, '16:9 Top-Left + BgBox', black16x9, 1280, 720, 'br');

  // Case 3: 720x1280 portrait multi-line text centered
  await page.getByTestId('input-edit-path').fill(black9x16);
  await page.getByTestId('btn-probe-edit').click();
  await expect(page.getByTestId('edit-probe-meta')).toContainText('720x1280', { timeout: 10_000 });
  await page.getByTestId('layer-row-' + await page.locator('[data-testid^="layer-row-"]').first().getAttribute('data-testid').then((v) => v!.replace('layer-row-', ''))).click();
  await page.getByTestId('toggle-text-bg').click();
  await page.getByTestId('align-pt-mc').click();
  await page.getByTestId('input-text-content').fill('GIAM 50%\nCHOT DON');
  const c3 = await renderAndMeasureCase(page, '9:16 Multi-Line Center', black9x16, 720, 1280, 'tl');

  for (const r of [c1, c2, c3]) {
    expect(r.preview.litPixels).toBeGreaterThan(50);
    expect(r.render.litPixels).toBeGreaterThan(50);
    expect(r.dcx).toBeLessThanOrEqual(3);
    expect(r.dcy).toBeLessThanOrEqual(3);
    expect(r.dw).toBeLessThanOrEqual(4);
    expect(r.dh).toBeLessThanOrEqual(4);
    expect(r.iou).toBeGreaterThanOrEqual(90);
  }
});

test('pixel-level parity for Image overlays (16:9 & 9:16)', async ({ page }) => {
  test.setTimeout(90_000);
  await page.goto('/');

  await page.getByTestId('input-edit-path').fill(black16x9);
  await page.getByTestId('btn-probe-edit').click();
  await expect(page.getByTestId('edit-probe-meta')).toBeVisible({ timeout: 10_000 });

  await page.getByTestId('btn-toggle-image-path').click();
  await page.getByTestId('input-image-layer-path').fill(logoPng);
  await page.getByTestId('btn-confirm-image-path').click();
  await expect(page.getByTestId('input-image-width')).toBeVisible();

  await page.getByTestId('align-pt-mc').click();
  const i1 = await renderAndMeasureCase(page, 'Image 16:9 Center (20%)', black16x9, 1280, 720, 'tl');

  const row = page.locator('[data-testid^="layer-row-"]').first();
  await row.click();
  await page.getByTestId('align-pt-tl').click();
  const i2 = await renderAndMeasureCase(page, 'Image 16:9 Top-Left (20%)', black16x9, 1280, 720, 'br');

  await page.getByTestId('input-edit-path').fill(black9x16);
  await page.getByTestId('btn-probe-edit').click();
  await expect(page.getByTestId('edit-probe-meta')).toContainText('720x1280', { timeout: 10_000 });
  await row.click();
  await page.getByTestId('input-image-width').fill('25');
  await page.getByTestId('input-image-width').dispatchEvent('input');
  await page.getByTestId('align-pt-br').click();
  const i3 = await renderAndMeasureCase(page, 'Image 9:16 Bottom-Right (25%)', black9x16, 720, 1280, 'tl');

  for (const r of [i1, i2, i3]) {
    expect(r.preview.litPixels).toBeGreaterThan(500);
    expect(r.render.litPixels).toBeGreaterThan(500);
    expect(r.dcx).toBeLessThanOrEqual(4);
    expect(r.dcy).toBeLessThanOrEqual(4);
    expect(r.dw).toBeLessThanOrEqual(4);
    expect(r.dh).toBeLessThanOrEqual(4);
  }
});
