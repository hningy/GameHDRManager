#!/usr/bin/env python3
"""
Game HDR Manager - 自动管理 Windows HDR 模式
监控指定游戏进程，游戏启动时自动开启 HDR，游戏退出时自动关闭 HDR。

依赖: 无（使用 Windows 内置 API）
快捷键: Win+Alt+B (Windows 11 原生 HDR 切换快捷键)
"""

import ctypes
import json
import os
import subprocess
import sys
import time
import logging
from datetime import datetime
from pathlib import Path

# ============================================================
# 日志配置
# ============================================================
SCRIPT_DIR = Path(__file__).parent
LOG_FILE = SCRIPT_DIR / "hdr_manager.log"

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S',
    handlers=[
        logging.FileHandler(LOG_FILE, encoding='utf-8'),
        logging.StreamHandler(sys.stdout)
    ]
)
log = logging.getLogger(__name__)

# ============================================================
# Windows API - 键盘模拟 (SendInput 替代方案)
# ============================================================
VK_LWIN = 0x5B   # 左 Win 键
VK_LMENU = 0xA4  # 左 Alt 键
VK_B = 0x42      # B 键

KEYEVENTF_KEYUP = 0x0002
KEYEVENTF_EXTENDEDKEY = 0x0001

user32 = ctypes.windll.user32


def send_win_alt_b():
    """通过 keybd_event 发送 Win+Alt+B 快捷键来切换 HDR。"""
    log.info("发送 Win+Alt+B 快捷键...")

    # 按下
    user32.keybd_event(VK_LWIN, 0, KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.03)
    user32.keybd_event(VK_LMENU, 0, KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.03)
    user32.keybd_event(VK_B, 0, 0, 0)
    time.sleep(0.08)

    # 释放（逆序）
    user32.keybd_event(VK_B, 0, KEYEVENTF_KEYUP, 0)
    time.sleep(0.03)
    user32.keybd_event(VK_LMENU, 0, KEYEVENTF_KEYUP | KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.03)
    user32.keybd_event(VK_LWIN, 0, KEYEVENTF_KEYUP | KEYEVENTF_EXTENDEDKEY, 0)

    # 等待系统响应
    time.sleep(1.0)


# ============================================================
# HDR 状态检测
# ============================================================
def is_hdr_enabled():
    """
    通过注册表检测 HDR 是否开启。

    Windows 11 的 HDR 状态存储在:
    HKCU\Software\Microsoft\Windows\CurrentVersion\VideoSettings\EnableHDR

    返回值: True (已开启), False (未开启), None (无法检测)
    """
    ps_cmd = (
        '$val = Get-ItemPropertyValue '
        '-Path "HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\VideoSettings" '
        '-Name "EnableHDR" '
        '-ErrorAction SilentlyContinue; '
        'if ($null -ne $val) { Write-Output $val } else { Write-Output "-1" }'
    )

    try:
        result = subprocess.run(
            ['powershell', '-NoProfile', '-Command', ps_cmd],
            capture_output=True,
            text=True,
            timeout=5,
            creationflags=subprocess.CREATE_NO_WINDOW
        )
        value = result.stdout.strip()
        if value == '1':
            return True
        elif value == '0':
            return False
        else:
            log.warning(f"无法读取 HDR 注册表值，返回: '{value}'")
            return None
    except subprocess.TimeoutExpired:
        log.error("检测 HDR 状态超时")
        return None
    except Exception as e:
        log.error(f"检测 HDR 状态出错: {e}")
        return None


# ============================================================
# 进程检测
# ============================================================
def check_process_running(process_name):
    """通过 tasklist 检测指定进程是否在运行。"""
    try:
        result = subprocess.run(
            ['tasklist', '/FI', f'IMAGENAME eq {process_name}', '/NH'],
            capture_output=True,
            text=True,
            timeout=5,
            creationflags=subprocess.CREATE_NO_WINDOW
        )
        # tasklist 返回 "INFO: No tasks..." 表示未找到
        if 'No tasks' in result.stdout or '没有运行' in result.stdout:
            return False
        return process_name.lower() in result.stdout.lower()
    except Exception as e:
        log.error(f"检测进程 {process_name} 出错: {e}")
        return False


def get_running_games(games_config):
    """返回当前正在运行的游戏列表。"""
    running = []
    for game in games_config:
        if check_process_running(game['process']):
            running.append(game)
    return running


# ============================================================
# 配置加载
# ============================================================
def load_config(config_path):
    """加载游戏配置文件，如果不存在则创建默认配置。"""
    default_config = {
        "games": [
            {"name": "示例: 赛博朋克2077", "process": "Cyberpunk2077.exe"},
            {"name": "示例: 艾尔登法环", "process": "eldenring.exe"},
        ],
        "settings": {
            "poll_interval_seconds": 5,
            "restore_hdr_on_exit": True,
            "verbose_logging": False
        }
    }

    if not os.path.exists(config_path):
        log.warning(f"配置文件不存在: {config_path}")
        log.warning("已创建默认配置文件，请编辑后重新运行。")
        with open(config_path, 'w', encoding='utf-8') as f:
            json.dump(default_config, f, indent=2, ensure_ascii=False)
        return default_config

    with open(config_path, 'r', encoding='utf-8') as f:
        config = json.load(f)

    # 合并默认值
    if 'settings' not in config:
        config['settings'] = {}
    config['settings'].setdefault('poll_interval_seconds', 5)
    config['settings'].setdefault('restore_hdr_on_exit', True)
    config['settings'].setdefault('verbose_logging', False)

    return config


def format_game_list(games):
    """格式化游戏列表用于显示。"""
    lines = []
    for i, game in enumerate(games, 1):
        lines.append(f"  {i}. {game['name']} ({game['process']})")
    return '\n'.join(lines) if lines else '  (无)'


# ============================================================
# 主循环
# ============================================================
def main():
    config_path = SCRIPT_DIR / "games_config.json"

    print("=" * 55)
    print("  🎮 Game HDR Manager - Windows HDR 自动切换工具")
    print("=" * 55)
    log.info("Game HDR Manager 启动中...")

    # 加载配置
    config = load_config(config_path)
    games = config['games']
    settings = config['settings']
    poll_interval = settings['poll_interval_seconds']

    # 过滤掉示例条目（以 "示例:" 开头的）
    actual_games = [g for g in games if not g['name'].startswith('示例:')]

    if not actual_games:
        print("\n⚠️  没有配置任何游戏！")
        print(f"   请编辑配置文件: {config_path}")
        print("   添加你的游戏进程名（可在任务管理器中查看）")
        print("\n   按 Enter 键退出...")
        input()
        sys.exit(1)

    print(f"\n📋 监控列表 ({len(actual_games)} 个游戏):")
    print(format_game_list(actual_games))
    print(f"\n⏱️  轮询间隔: {poll_interval} 秒")
    print(f"🔄 退出时恢复 HDR: {'是' if settings['restore_hdr_on_exit'] else '否'}")
    print(f"📝 日志文件: {LOG_FILE}")
    print("\n💡 按 Ctrl+C 退出程序\n")

    log.info(f"开始监控 {len(actual_games)} 个游戏")
    log.info(f"轮询间隔: {poll_interval}s, 退出恢复: {settings['restore_hdr_on_exit']}")

    # ============================================================
    # 状态跟踪
    # ============================================================
    hdr_toggled_by_us = False   # 是否由本程序开启了 HDR
    hdr_was_on_before = None    # 游戏启动前 HDR 是否已经开启
    was_gaming = False           # 上一轮是否有游戏运行
    running_game_names = ""      # 当前运行的游戏名（用于日志）

    try:
        while True:
            # 检测当前运行的游戏
            running_games = get_running_games(actual_games)
            is_gaming = len(running_games) > 0
            current_game_names = ', '.join(g['name'] for g in running_games)

            # ---------- 状态变化：无游戏 → 有游戏 ----------
            if is_gaming and not was_gaming:
                print(f"\n🎮 [{datetime.now().strftime('%H:%M:%S')}] 检测到游戏启动: {current_game_names}")
                log.info(f"游戏启动: {current_game_names}")

                # 检测当前 HDR 状态
                hdr_state = is_hdr_enabled()
                state_str = '已开启' if hdr_state else ('已关闭' if hdr_state is False else '未知')
                log.info(f"当前 HDR 状态: {state_str}")

                if hdr_state is False:
                    print("   💡 HDR 未开启，正在开启...")
                    send_win_alt_b()
                    hdr_toggled_by_us = True
                    hdr_was_on_before = False

                    # 验证
                    time.sleep(1)
                    new_state = is_hdr_enabled()
                    if new_state:
                        print("   ✅ HDR 已开启")
                        log.info("HDR 开启成功")
                    else:
                        print("   ⚠️ HDR 状态未变化，可能快捷键未生效")
                        log.warning("HDR 切换后状态未变化，请检查 Win+Alt+B 是否可用")
                elif hdr_state is True:
                    print("   ℹ️ HDR 已经开启，无需操作")
                    hdr_toggled_by_us = False
                    hdr_was_on_before = True
                else:
                    print("   ⚠️ 无法检测 HDR 状态，尝试切换...")
                    send_win_alt_b()
                    hdr_toggled_by_us = True
                    hdr_was_on_before = None

            # ---------- 状态变化：有游戏 → 无游戏 ----------
            elif not is_gaming and was_gaming:
                print(f"\n🏁 [{datetime.now().strftime('%H:%M:%S')}] 所有游戏已退出")
                log.info("所有游戏已退出")

                if hdr_toggled_by_us and settings['restore_hdr_on_exit']:
                    print("   🔄 正在恢复 HDR 到游戏前状态（关闭）...")
                    send_win_alt_b()
                    hdr_toggled_by_us = False

                    time.sleep(1)
                    new_state = is_hdr_enabled()
                    if new_state is False:
                        print("   ✅ HDR 已关闭")
                        log.info("HDR 已恢复关闭")
                    else:
                        print("   ⚠️ HDR 状态异常，请手动按 Win+Alt+B")
                        log.warning("HDR 恢复关闭失败")
                elif hdr_was_on_before:
                    print("   ℹ️ HDR 在游戏前已开启，保持不变")
                else:
                    print("   ℹ️ HDR 非本程序开启，保持不变")

                hdr_was_on_before = None
                running_game_names = ""

            # ---------- 持续游戏状态 ----------
            elif is_gaming and was_gaming:
                if settings.get('verbose_logging'):
                    log.debug(f"游戏中... ({current_game_names})")

            was_gaming = is_gaming
            running_game_names = current_game_names if is_gaming else ""

            time.sleep(poll_interval)

    except KeyboardInterrupt:
        print("\n\n" + "=" * 55)
        print("  正在退出...")
        log.info("收到退出信号，正在关闭...")

        # 安全检查：如果 HDR 是我们开启的，提醒用户
        if hdr_toggled_by_us and is_gaming:
            print("   ⚠️ HDR 由本程序开启，游戏可能仍在运行")
            print("   如需手动关闭 HDR，请按 Win+Alt+B")
            log.warning("退出时 HDR 仍处于程序开启状态")

        print("   👋 再见！")
        print("=" * 55)
        log.info("Game HDR Manager 已退出")

    except Exception as e:
        log.error(f"未预期的错误: {e}", exc_info=True)
        print(f"\n❌ 发生错误: {e}")
        print("   详情请查看日志文件")
        print("   按 Enter 键退出...")
        input()


if __name__ == '__main__':
    main()
