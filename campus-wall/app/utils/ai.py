# -*- coding: utf-8 -*-
"""AI 适配器接口（R12）。

接口约定（可插拔）：
    provider.moderate(title, content) -> {'action': 'approve'|'reject'|'review', 'reason': str}
    provider.reply(text, context=None) -> str

默认 RuleBasedProvider：纯本地规则引擎（零依赖、离线可用、结果可复现）。
若设置 AI_API_KEY / AI_BASE_URL 环境变量，则切换 OpenAI 兼容 Chat API 适配器；
调用失败自动回落规则引擎，保证功能永远可用。
"""
import os


class RuleBasedProvider:
    """本地规则引擎（默认实现）。"""

    name = 'rule-based'

    def moderate(self, title, content):
        from app.models_ext import ai_moderate
        return ai_moderate(title, content)

    def reply(self, text, context=None):
        from app.models_ext import ai_reply_text
        return ai_reply_text(text)


class OpenAICompatProvider:
    """OpenAI 兼容 Chat API 适配器（AI_API_KEY + AI_BASE_URL 可选启用）。"""

    name = 'openai-compat'

    def __init__(self, api_key, base_url, model=None):
        self.api_key = api_key
        self.base_url = (base_url or 'https://api.openai.com/v1').rstrip('/')
        self.model = model or os.environ.get('AI_MODEL', 'gpt-4o-mini')

    def _chat(self, system, user_text):
        import json as _json
        import urllib.request
        body = _json.dumps({
            'model': self.model,
            'messages': [
                {'role': 'system', 'content': system},
                {'role': 'user', 'content': user_text},
            ],
            'temperature': 0.3,
        }).encode()
        req = urllib.request.Request(
            self.base_url + '/chat/completions', data=body, method='POST',
            headers={'Content-Type': 'application/json', 'Authorization': 'Bearer ' + self.api_key})
        with urllib.request.urlopen(req, timeout=20) as resp:
            data = _json.loads(resp.read().decode())
        return data['choices'][0]['message']['content']

    def moderate(self, title, content):
        try:
            out = self._chat(
                '你是校园社区审核员。仅输出 JSON：{"action":"approve|reject|review","reason":"简短理由"}',
                f'标题：{title}\n内容：{content}')
            import json as _json
            data = _json.loads(out[out.index('{'):out.rindex('}') + 1])
            if data.get('action') in ('approve', 'reject', 'review'):
                return {'action': data['action'], 'reason': data.get('reason', '')}
        except Exception:
            pass
        return RuleBasedProvider().moderate(title, content)

    def reply(self, text, context=None):
        try:
            return self._chat('你是校园墙的友好 AI 助手「小智」，用一句轻松的中文回复。', text)
        except Exception:
            return RuleBasedProvider().reply(text)


def get_provider():
    api_key = os.environ.get('AI_API_KEY')
    if api_key:
        try:
            return OpenAICompatProvider(api_key, os.environ.get('AI_BASE_URL'))
        except Exception:
            pass
    return RuleBasedProvider()


def provider_name():
    return get_provider().name
