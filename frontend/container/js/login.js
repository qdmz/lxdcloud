/* 登录页逻辑 */
(function () {
  let captchaId = '';
  const hashInput = document.getElementById('hashInput');
  const captchaInput = document.getElementById('captchaInput');
  const captchaImg = document.getElementById('captchaImg');
  const submitBtn = document.getElementById('submitBtn');

  // 品牌信息
  LXD.loadBrand().then((brand) => {
    LXD.applyBrand(brand);
    if (brand.footer_text) {
      document.getElementById('footerText').textContent = brand.footer_text;
    }
  }).catch(() => {});

  // 预填 URL 中的 hash
  const params = new URLSearchParams(window.location.search);
  const urlHash = params.get('hash');
  if (urlHash) {
    hashInput.value = urlHash;
    captchaInput.focus();
  }
  // 已有 localStorage hash 则预填
  const saved = localStorage.getItem('container_hash');
  if (saved && !hashInput.value) hashInput.value = saved;

  // 加载验证码
  function loadCaptcha() {
    captchaImg.style.opacity = '0.5';
    LXD.request('/api/container/captcha').then((res) => {
      const d = (res.data && typeof res.data === 'object') ? res.data : res;
      if (d.code === 200 && d.image) {
        captchaId = d.captcha_id;
        captchaImg.src = (d.image && d.image.startsWith('data:')) ? d.image : 'data:image/png;base64,' + d.image;
      } else {
        LXD.toast('error', d.msg || '验证码获取失败');
      }
    }).catch(() => LXD.toast('error', '验证码获取失败，请检查服务')).finally(() => {
      captchaImg.style.opacity = '1';
    });
  }
  captchaImg.onclick = loadCaptcha;
  loadCaptcha();

  // 提交验证
  hashInput.addEventListener('input', () => {
    hashInput.value = hashInput.value.replace(/\s+/g, '');
  });

  document.getElementById('loginForm').addEventListener('submit', (e) => {
    e.preventDefault();
    const hash = hashInput.value.trim();
    const captcha = captchaInput.value.trim();
    if (!hash) return LXD.toast('warning', '请输入容器访问码');
    if (!captcha) return LXD.toast('warning', '请输入验证码');

    submitBtn.disabled = true;
    submitBtn.textContent = '验证中...';

    LXD.request('/api/container/verify', {
      method: 'POST',
      body: { hash, captcha }
    }).then((res) => {
      if (res.code === 200) {
        localStorage.setItem('container_hash', hash);
        LXD.toast('success', '验证成功，正在进入...');
        setTimeout(() => { window.location.href = 'dashboard.html'; }, 500);
      } else {
        LXD.toast('error', res.msg || '验证失败');
        captchaInput.value = '';
        loadCaptcha();
      }
    }).catch((err) => {
      LXD.toast('error', err.message || '验证失败');
      captchaInput.value = '';
      loadCaptcha();
    }).finally(() => {
      submitBtn.disabled = false;
      submitBtn.textContent = '进入控制面板';
    });
  });
})();
