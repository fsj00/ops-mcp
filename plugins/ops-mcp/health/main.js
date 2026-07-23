function execute(ctx) {
  var base = ctx.params.base_url || "http://127.0.0.1:20267";
  // 去掉末尾斜杠，避免 //health
  if (base.charAt(base.length - 1) === "/") {
    base = base.substring(0, base.length - 1);
  }
  // URL 直连模式：不依赖 apis.yaml
  var res = ctx.http.get({
    url: base + "/health",
    timeout: "5s"
  });
  return {
    status_code: res.status_code,
    body: res.body,
    ok: res.status_code === 200
  };
}
