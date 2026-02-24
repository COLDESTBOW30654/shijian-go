<?php
$statusFile = __DIR__ . '/status.json';
$logFile = __DIR__ . '/debug.log';
$timeoutSeconds = 40;

// 记录调试日志
function writeDebugLog($msg) {
    global $logFile;
    $time = date('Y-m-d H:i:s');
    $content = "[$time] " . $msg . PHP_EOL;
    file_put_contents($logFile, $content, FILE_APPEND);
}

// 调试模式
if (isset($_GET['debug'])) {
    header('Content-Type: text/plain; charset=utf-8');
    if (file_exists($logFile)) {
        echo "=== 调试日志 ===" . PHP_EOL;
        echo file_get_contents($logFile);
    } else {
        echo "暂无日志";
    }
    exit;
}

// 处理 Go 客户端推送
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $json = file_get_contents('php://input');
    $data = json_decode($json, true);
    if ($data) {
        $data['last_seen'] = time();
        file_put_contents($statusFile, json_encode($data), LOCK_EX);
        writeDebugLog("收到更新: " . $data['process_name'] . " 状态: " . $data['status']);
        exit("Success");
    }
    writeDebugLog("收到错误请求数据");
    exit("Invalid Data");
}

// 处理前端 Fetch 请求
if (isset($_GET['fetch'])) {
    header('Content-Type: application/json');
    echo file_exists($statusFile) ? file_get_contents($statusFile) : json_encode([]);
    exit;
}

// 页面初始加载变量
$data = file_exists($statusFile) ? json_decode(file_get_contents($statusFile), true) : null;
$displayStatus = "空闲中";
$dotColor = "#7bba7b";
$isActive = false;

if ($data) {
    $lastSeen = $data['last_seen'] ?? 0;
    if ((time() - $lastSeen) < $timeoutSeconds && $data['status'] === 'active') {
        $isActive = true;
        $dotColor = "#ff4757";
    }
}

// 这段代码可加可不加
// 防止他人直接访问接口页面
if (realpath(__FILE__) === realpath($_SERVER['SCRIPT_FILENAME'])) {
    http_response_code(404);
}