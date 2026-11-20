import { expect, test } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

let tempDir = '';
let portraitVideo = '';
let landscapeVideo = '';

function findFfmpeg(): string {
  const wingetPath =
    'C:\\Users\\8hung\\AppData\\Local\\Microsoft\\WinGet\\Packages\\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\\ffmpeg-8.1.1-full_build\\bin\\ffmpeg.exe';
  if (fs.existsSync(wingetPath)) return wingetPath;
  return 'ffmpeg';
}

test.beforeAll(() => {
  tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 've-e2e-'));
  const ffmpeg = findFfmpeg();
  const envPortrait = process.env.E2E_VIDEO_PORTRAIT;
  const knownPortrait = 'D:\\system\\Downloads\\LLM-Vtuber.mp4';

  if (envPortrait && fs.existsSync(envPortrait)) {
    portraitVideo = envPortrait;
  } else if (fs.existsSync(knownPortrait)) {
    portraitVideo = knownPortrait;
  } else {
    portraitVideo = path.join(tempDir, 'portrait_9x16.mp4');
    execFileSync(ffmpeg, [
      '-y', '-f', 'lavfi', '-i', 'testsrc2=size=360x640:rate=30:duration=3',
      '-f', 'lavfi', '-i', 'sine=frequency=440:duration=3',
      '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-c:a', 'aac', portraitVideo
    ]);
  }

  const envLandscape = process.env.E2E_VIDEO_LANDSCAPE;
  if (envLandscape && fs.existsSync(envLandscape)) {
    landscapeVideo = envLandscape;
  } else {
    landscapeVideo = path.join(tempDir, 'landscape_16x9.mp4');
    execFileSync(ffmpeg, [
      '-y', '-f', 'lavfi', '-i', 'testsrc2=size=640x360:rate=30:duration=3',
      '-f', 'lavfi', '-i', 'sine=frequency=520:duration=3',
      '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-c:a', 'aac', landscapeVideo
    ]);
  }
});

test.afterAll(() => {
  if (tempDir && fs.existsSync(tempDir)) {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
});

test('header GPU badge reflects capabilities and /api/status matches schema', async ({ page }) => {
  const capsRes = await page.request.get('/api/system/capabilities');
  expect(capsRes.ok()).toBeTruthy();
  const caps = await capsRes.json();

  const statusRes = await page.request.get('/api/status');
  expect(statusRes.ok()).toBeTruthy();
  const status = await statusRes.json();
  for (const key of ['status', 'mode', 'progress', 'fps', 'speed', 'time_processed', 'eta', 'current_step', 'active_encoder', 'output_files', 'warnings', 'logs']) {
    expect(status).toHaveProperty(key);
  }

  await page.goto('/');
  const badge = page.getByTestId('gpu-status-badge');
  await expect(badge).toBeVisible({ timeout: 15_000 });
  await expect(badge).not.toContainText('Đang kiểm tra GPU...');
  if (caps.has_nvidia_gpu && (caps.nvenc_h264 || caps.nvenc_hevc)) {
    await expect(badge).toContainText('NVENC Ready');
  } else {
    await expect(badge).toContainText(/CPU|FFmpeg/);
  }
});

test('9:16 Konva geometry, icon text controls, 9-pt align, zero values, and quality mode', async ({ page }) => {
  const pageErrors: string[] = [];
  page.on('pageerror', (err) => pageErrors.push(err.message));

  await page.goto('/');
  await page.getByTestId('input-edit-path').fill(portraitVideo);
  await page.getByTestId('btn-probe-edit').click();

  await expect(page.getByTestId('edit-probe-meta')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('probe-bitrate-badge')).toContainText('kbps');

  const viewportBox = await page.getByTestId('monitor-viewport').boundingBox();
  const stageBox = await page.getByTestId('konva-stage-container').boundingBox();
  expect(viewportBox).not.toBeNull();
  expect(stageBox).not.toBeNull();
  if (viewportBox && stageBox) {
    expect(stageBox.width).toBeLessThan(stageBox.height);
    expect(stageBox.width).toBeLessThan(viewportBox.width);
    expect(stageBox.x).toBeGreaterThan(viewportBox.x);
  }

  // Add text layer + test multiline %, S/M/L/XL, opacity=0, bgPadding=0, 9-pt align, timing bar
  await page.getByTestId('btn-add-text-layer').click();
  await page.getByTestId('input-text-content').fill('Giảm 50%\nHôm nay');
  await expect(page.getByTestId('input-text-content')).toHaveValue('Giảm 50%\nHôm nay');

  await page.getByTestId('btn-font-XL').click();
  await expect(page.getByTestId('text-size-display')).toHaveText('72px');
  await page.getByTestId('btn-font-dec').click();
  await expect(page.getByTestId('text-size-display')).toHaveText('68px');

  await page.getByTestId('input-text-opacity').fill('0');
  await page.getByTestId('input-text-opacity').dispatchEvent('input');
  await expect(page.getByText('0%', { exact: true }).first()).toBeVisible();

  await page.getByTestId('toggle-text-bg').check();
  await page.getByTestId('input-text-bg-padding').fill('0');
  await page.getByTestId('input-text-bg-padding').dispatchEvent('input');
  await expect(page.getByText('Đệm (0px)')).toBeVisible();

  // 9-point alignment grid
  await page.getByTestId('align-pt-tl').click();
  await expect(page.getByText('x:15% y:15%')).toBeVisible();
  await page.getByTestId('align-pt-mc').click();
  await expect(page.getByText('x:50% y:50%')).toBeVisible();

  // Timing bar Out at playhead 0 -> 0% - 0%, then Full -> 0% - 100%
  await page.getByTestId('btn-layer-out').click();
  await expect(page.getByTestId('layer-span-badge')).toHaveText('0% - 0%');
  await page.getByTestId('btn-layer-full-span').click();
  await expect(page.getByTestId('layer-span-badge')).toHaveText('0% - 100%');

  // Filter + Trim + Speed 2x
  await page.getByTestId('select-video-filter').selectOption('cool');
  const filterStyle = await page.getByTestId('preview-video').evaluate((el) => (el as HTMLElement).style.filter);
  expect(filterStyle).toContain('ve-filter-cool');

  await page.getByTestId('toggle-trim').check();
  await page.getByTestId('btn-add-trim').click();
  await page.getByRole('button', { name: '2x', exact: true }).click();
  await expect(page.getByTestId('speed-val-display')).toHaveText('2.00x');
  await expect(page.getByTestId('output-duration-badge')).toContainText('(2.00x)');
  expect(pageErrors).toEqual([]);
});

test('NLE shortcuts (Ctrl+B, Ctrl+C/V, [/], zoom) and multi-track timeline', async ({ page }) => {
  const pageErrors: string[] = [];
  page.on('pageerror', (err) => pageErrors.push(err.message));

  await page.goto('/');
  await page.getByTestId('input-edit-path').fill(landscapeVideo);
  await page.getByTestId('btn-probe-edit').click();
  await expect(page.getByTestId('timeline-track-video')).toBeVisible({ timeout: 15_000 });

  // Press T to add text layer -> creates layer track row
  await page.keyboard.press('t');
  await expect(page.getByTestId('layers-list').locator('> div')).toHaveCount(1);
  await expect(page.locator('[data-testid^="layer-clip-"]')).toHaveCount(1);

  // Focus input guard: Ctrl+B inside textarea must NOT trigger trim cut
  await page.getByTestId('input-text-content').focus();
  await page.keyboard.press('Control+b');
  await expect(page.getByTestId('trim-chips-list')).toHaveCount(0);
  await page.getByTestId('input-text-content').evaluate((el) => (el as HTMLElement).blur());

  // Inline text edit via dblclick on Konva stage center
  const stageBox = await page.getByTestId('konva-stage-container').boundingBox();
  expect(stageBox).not.toBeNull();
  if (stageBox) {
    await page.mouse.dblclick(stageBox.x + stageBox.width / 2, stageBox.y + stageBox.height / 2);
    const inlineEditor = page.getByTestId('inline-text-editor');
    await expect(inlineEditor).toBeVisible();
    await inlineEditor.fill('Live Edit OK');
    await inlineEditor.press('Enter');
    await expect(page.getByTestId('input-text-content')).toHaveValue('Live Edit OK');
  }

  // Ctrl+C then Ctrl+V duplicates layer
  await page.keyboard.press('Control+c');
  await page.keyboard.press('Control+v');
  await expect(page.getByTestId('layers-list').locator('> div')).toHaveCount(2);
  await expect(page.locator('[data-testid^="layer-clip-"]')).toHaveCount(2);

  // Step playhead forward, then press Ctrl+B -> creates & selects trim clip
  await page.getByTestId('btn-step-fwd').click();
  await page.keyboard.press('Control+b');
  await expect(page.getByTestId('trim-chips-list').locator('> div')).toHaveCount(1);
  await expect(page.locator('[data-testid^="trim-clip-"]')).toHaveCount(1);

  // Set In-point with [ after stepping forward again
  await page.getByTestId('btn-step-fwd').click();
  await page.keyboard.press('[');

  // Zoom In via toolbar & Fit reset via Ctrl+0
  await page.getByTestId('btn-zoom-in').click();
  await expect(page.getByTestId('timeline-zoom-label')).toHaveText('1.5x');
  await page.keyboard.press('Control+0');
  await expect(page.getByTestId('timeline-zoom-label')).toHaveText('1x');

  // Delete selected trim clip via Delete shortcut
  await page.keyboard.press('Delete');
  await expect(page.getByTestId('trim-chips-list').locator('> div')).toHaveCount(0);

  // Real mouse drag UPWARD on Konva stage (tests first-press drag without node destruction)
  if (stageBox) {
    const cx = stageBox.x + stageBox.width * 0.53;
    const cy = stageBox.y + stageBox.height * 0.53;
    await page.mouse.move(cx, cy);
    await page.mouse.down();
    await page.mouse.move(cx, stageBox.y + stageBox.height * 0.2, { steps: 10 });
    await page.mouse.up();
    await expect(page.getByTestId('text-coords-label')).toContainText(/y:(1\d|2\d|3[0-5])%/);
  }

  // Real mouse drag UPWARD on timeline resize handle
  const resizeBox = await page.getByTestId('timeline-resize-handle').boundingBox();
  const scrollBefore = await page.getByTestId('timeline-scroll-area').boundingBox();
  expect(resizeBox).not.toBeNull();
  expect(scrollBefore).not.toBeNull();
  if (resizeBox && scrollBefore) {
    const rx = resizeBox.x + resizeBox.width / 2;
    const ry = resizeBox.y + resizeBox.height / 2;
    await page.mouse.move(rx, ry);
    await page.mouse.down();
    await page.mouse.move(rx, ry - 60, { steps: 6 });
    await page.mouse.up();
    const scrollAfter = await page.getByTestId('timeline-scroll-area').boundingBox();
    expect(scrollAfter!.height).toBeGreaterThan(scrollBefore.height + 30);
  }
  expect(pageErrors).toEqual([]);
});

test('split markers, uniform presets, and merge ConfirmModal work cleanly', async ({ page }) => {
  const pageErrors: string[] = [];
  page.on('pageerror', (err) => pageErrors.push(err.message));

  await page.goto('/');
  await page.getByTestId('tab-split').click();
  await page.getByTestId('input-split-path').fill(landscapeVideo);
  await page.getByTestId('btn-probe-split').click();

  await expect(page.getByTestId('btn-add-split-marker')).toBeVisible({ timeout: 15_000 });
  await page.getByTestId('btn-add-split-marker').click();
  await page.getByTestId('btn-add-split-marker').click();
  await expect(page.getByTestId('split-segments-list').locator('> div')).toHaveCount(3);
  await expect(page.getByTestId('btn-start-split')).toBeEnabled();

  await page.getByTestId('btn-clear-split-markers').click();
  await expect(page.getByTestId('split-segments-list').locator('> div')).toHaveCount(1);

  // Uniform split icon presets
  await page.getByTestId('split-mode-uniform').click();
  await page.getByTestId('preset-split-parts-5').click();
  await expect(page.getByTestId('split-parts-value')).toHaveText('5 phần');
  await expect(page.getByTestId('split-segments-list').locator('> div')).toHaveCount(5);

  // Merge tab UI & ConfirmModal
  await page.getByTestId('tab-merge').click();
  await page.getByTestId('input-merge-path').fill(landscapeVideo);
  await page.getByTestId('btn-add-merge-path').click();
  await page.getByTestId('input-merge-path').fill(portraitVideo);
  await page.getByTestId('btn-add-merge-path').click();

  await expect(page.getByTestId('merge-count-label')).toHaveText('2 video đã thêm');
  await expect(page.getByTestId('btn-start-merge')).toBeEnabled();

  await page.getByTestId('btn-clear-merge').click();
  await page.getByRole('dialog').getByRole('button', { name: 'Xóa tất cả' }).click();
  await expect(page.getByTestId('merge-count-label')).toHaveText('0 video đã thêm');
  await expect(page.getByTestId('btn-start-merge')).toBeDisabled();
  expect(pageErrors).toEqual([]);
});
