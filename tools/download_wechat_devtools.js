const fs = require('fs');
const https = require('https');

const outputPath = 'D:/微信小游戏/downloads/wechat_devtools_2.01.2601082_win32_x64.exe';
const downloadUrl = 'https://dldir1.qq.com/WechatWebDev/nightly/p-3bd19c2db3a642a0b39af853efaf67f8/0.54.1/wechat_devtools_2.01.2601082_win32_x64.exe';

fs.mkdirSync('D:/微信小游戏/downloads', { recursive: true });

function download(url, redirects = 0) {
  if (redirects > 5) throw new Error('下载重定向次数过多');
  return new Promise((resolve, reject) => {
    const request = https.get(url, { headers: { 'User-Agent': 'Mozilla/5.0' } }, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode) && response.headers.location) {
        response.resume();
        download(new URL(response.headers.location, url).toString(), redirects + 1).then(resolve, reject);
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`下载失败，HTTP ${response.statusCode}`));
        return;
      }
      const file = fs.createWriteStream(outputPath);
      response.pipe(file);
      file.on('finish', () => file.close(resolve));
      file.on('error', reject);
    });
    request.on('error', reject);
  });
}

download(downloadUrl).then(() => {
  const stat = fs.statSync(outputPath);
  console.log(JSON.stringify({ path: outputPath, sizeMB: +(stat.size / 1024 / 1024).toFixed(2) }));
}).catch((error) => {
  try { fs.unlinkSync(outputPath); } catch (_) {}
  console.error(error.message);
  process.exit(1);
});
