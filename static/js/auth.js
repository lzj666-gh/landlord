//登录 / 注册表单：房间列表和牌桌页共用，不依赖 Phaser
(function () {
    var bound = false;

    var messages = {
        invalid_credentials: '用户名或密码错误',
        username_taken: '该用户名已经被占用',
        password_too_long: '密码不能超过72个字节',
        missing_credentials: '请输入用户名和密码'
    };

    var Auth = {
        isRegister: false,
        bindForm: function () {
            if (bound) {
                return;
            }
            bound = true;
            this.form = document.getElementById('auth-form');
            this.username = document.getElementById('auth-username');
            this.password = document.getElementById('auth-password');
            this.passwordAgain = document.getElementById('auth-password-again');
            this.confirmRow = document.getElementById('auth-confirm-row');
            this.errorText = document.getElementById('auth-error');
            this.actionButton = document.getElementById('auth-submit');
            this.toggleButton = document.getElementById('auth-toggle');
            this.form.addEventListener('submit', this.submit.bind(this));
            this.toggleButton.addEventListener('click', this.toggleMode.bind(this));
            this.username.focus();
        },

        toggleMode: function () {
            this.isRegister = !this.isRegister;
            this.confirmRow.hidden = !this.isRegister;
            this.password.autocomplete = this.isRegister ? 'new-password' : 'current-password';
            this.passwordAgain.required = this.isRegister;
            this.passwordAgain.value = '';
            this.errorText.textContent = '';
            this.actionButton.textContent = this.isRegister ? '注册' : '登录';
            this.toggleButton.textContent = this.isRegister ? '已有账号？返回登录' : '没有账号？立即注册';
        },

        submit: function (event) {
            event.preventDefault();
            if (this.isRegister && this.password.value !== this.passwordAgain.value) {
                this.errorText.textContent = '两次输入的密码不一致';
                return;
            }

            var httpRequest = new XMLHttpRequest();
            var auth = this;
            this.actionButton.disabled = true;
            this.toggleButton.disabled = true;
            httpRequest.onreadystatechange = function () {
                if (httpRequest.readyState !== XMLHttpRequest.DONE) {
                    return;
                }
                auth.actionButton.disabled = false;
                auth.toggleButton.disabled = false;
                var response;
                try {
                    response = JSON.parse(httpRequest.responseText);
                } catch (error) {
                    auth.errorText.textContent = '服务器响应异常，请稍后重试';
                    return;
                }
                if (httpRequest.status === 200 || httpRequest.status === 201) {
                    //登录成功后重新加载当前路由，由服务端把用户和房间信息渲染进来
                    window.location.reload();
                    return;
                }
                auth.errorText.textContent = messages[response.error] || '请求失败，请稍后重试';
            };
            httpRequest.onerror = function () {
                auth.actionButton.disabled = false;
                auth.toggleButton.disabled = false;
                auth.errorText.textContent = '网络连接失败，请稍后重试';
            };
            httpRequest.open('POST', this.isRegister ? '/reg' : '/login', true);
            httpRequest.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
            httpRequest.send('username=' + encodeURIComponent(this.username.value) +
                '&password=' + encodeURIComponent(this.password.value));
        }
    };

    window.PGAuth = Auth;
})();
