package iframewrap

const parentTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Gateway | My TON Storage</title>
<style>
  html,body{height:100%%;margin:0}
  .wrapped-iframe{width:100%%;height:calc(100%% - 50px);border:0px}
  .notice-header {
    background: linear-gradient(75deg, #dadadaff, #b3b3b3ff);
    padding: 8px 16px;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 13px;
    text-align: center;
    border-bottom: 1px solid #004080;
    position: relative;
    z-index: 1000;
    height: 34px;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .notice-text {
    opacity: 0.95;
  }
  .report-link {
	position:absolute;
	right:10px;
	top:8px;
	font-size:12px;
	text-decoration:none;
  }
  .warning-icon {
    margin-right: 8px;
    font-size: 14px;
  }
</style>
</head>
<body>
<div class="notice-header">
  <span class="warning-icon">⚠️</span>
  <span class="notice-text">This content is not part of <a href="https://mytonstorage.org">mytonstorage.org</a> website. Please be careful.</span>
  <a class="report-link" href="https://t.me/bagidreport_bot" target="_blank">Report</a>
</div>

<iframe class="wrapped-iframe" sandbox="%s" %s></iframe>
</body>
</html>`
