"""
Game HDR Manager - GUI
Auto-toggle Windows HDR when games start/stop.
Requires: Python 3 + tkinter (built-in)
"""

import ctypes
import json
import os
import re
import subprocess
import sys
import threading
import time
import tkinter as tk
from tkinter import ttk, messagebox, filedialog
from datetime import datetime
from pathlib import Path

SCRIPT_DIR = Path(__file__).parent
CONFIG_PATH = SCRIPT_DIR / "games_config.json"

VK_LWIN = 0x5B
VK_LMENU = 0xA4
VK_B = 0x42
KEYEVENTF_KEYUP = 0x0002
KEYEVENTF_EXTENDEDKEY = 0x0001

COLORS = {
    'bg_dark':      '#0d1117',
    'bg_panel':     '#161b22',
    'bg_card':      '#21262d',
    'bg_input':     '#0d1117',
    'bg_hover':     '#30363d',
    'border':       '#30363d',
    'text':         '#e6edf3',
    'text_dim':     '#8b949e',
    'text_muted':   '#484f58',
    'accent':       '#58a6ff',
    'accent_hover': '#79c0ff',
    'success':      '#3fb950',
    'success_dim':  '#238636',
    'warning':      '#d29922',
    'danger':       '#f85149',
    'hdr_on':       '#3fb950',
    'hdr_off':      '#484f58',
    'cyan':         '#39d2c0',
    'scrollbar':    '#30363d',
}

FONTS = {
    'title':   ('Segoe UI', 16, 'bold'),
    'heading': ('Segoe UI', 12, 'bold'),
    'body':    ('Segoe UI', 10),
    'small':   ('Segoe UI', 9),
    'mono':    ('Consolas', 10),
    'mono_sm': ('Consolas', 9),
}

# ========================================================
# HDR utilities
# ========================================================
_HDR_REG = r"Software\Microsoft\Windows\CurrentVersion\VideoSettings"

_user32 = None
def _u32():
    global _user32
    if _user32 is None:
        _user32 = ctypes.windll.user32
    return _user32

def _read_hdr():
    try:
        import winreg
        k = winreg.OpenKey(winreg.HKEY_CURRENT_USER, _HDR_REG, 0, winreg.KEY_READ)
        v, _ = winreg.QueryValueEx(k, "EnableHDR")
        winreg.CloseKey(k)
        return (v == 1, v)
    except FileNotFoundError:
        return (None, -1)
    except Exception as e:
        print("[HDR read error]", e)
        return (None, -1)
def _write_hdr(on):
    try:
        import winreg
        k = winreg.OpenKey(winreg.HKEY_CURRENT_USER, _HDR_REG, 0, winreg.KEY_SET_VALUE)
        winreg.SetValueEx(k, "EnableHDR", 0, winreg.REG_DWORD, 1 if on else 0)
        winreg.CloseKey(k)
        return True
    except Exception as e:
        print("[HDR write error]", e)
        return False

def _send_hotkey():
    """Send Win+Alt+B using keybd_event (virtual key codes)."""
    u = _u32()
    u.keybd_event(VK_LWIN, 0, KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.02)
    u.keybd_event(VK_LMENU, 0, KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.02)
    u.keybd_event(VK_B, 0, 0, 0)
    time.sleep(0.05)
    u.keybd_event(VK_B, 0, KEYEVENTF_KEYUP, 0)
    time.sleep(0.02)
    u.keybd_event(VK_LMENU, 0, KEYEVENTF_KEYUP | KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.02)
    u.keybd_event(VK_LWIN, 0, KEYEVENTF_KEYUP | KEYEVENTF_EXTENDEDKEY, 0)
    time.sleep(0.5)
def is_hdr_enabled():
    on, _ = _read_hdr()
    return on

def set_hdr_state(on, force=False):
    """Set HDR via registry write + display refresh. force=True always toggles."""
    target = bool(on)
    if not force:
        current, _ = _read_hdr()
        if current is not None and current == target:
            return current
    _write_hdr(target)
    import ctypes
    ctypes.windll.user32.ChangeDisplaySettingsExW(None, None, None, 0x40000000, None)
    _send_hotkey()
    time.sleep(1.5)
    new, _ = _read_hdr()
    if new == target:
        return target
    return current
def send_win_alt_b():
    set_hdr_state(not is_hdr_enabled())

def check_process(name):
    try:
        r = subprocess.run(
            ['tasklist', '/FI', 'IMAGENAME eq ' + name, '/NH'],
            capture_output=True, text=True, timeout=5,
            creationflags=subprocess.CREATE_NO_WINDOW
        )
        return name.lower() in r.stdout.lower() and 'No tasks' not in r.stdout
    except Exception:
        return False

def load_config():
    default = {
        "games": [],
        "settings": {
            "poll_interval_seconds": 2,
            "restore_hdr_on_exit": True,
            "start_monitoring_on_launch": False,
        }
    }
    if CONFIG_PATH.exists():
        try:
            with open(CONFIG_PATH, 'r', encoding='utf-8') as f:
                cfg = json.load(f)
            for k, v in default.items():
                cfg.setdefault(k, v)
            if 'settings' in cfg:
                for k, v in default['settings'].items():
                    cfg['settings'].setdefault(k, v)
            return cfg
        except Exception:
            return default
    return default

def save_config(cfg):
    with open(CONFIG_PATH, 'w', encoding='utf-8') as f:
        json.dump(cfg, f, indent=2, ensure_ascii=False)

# ========================================================
# Game scanner
# ========================================================
class GameScanner:

    @staticmethod
    def _get_steam_path():
        try:
            import winreg
            k = winreg.OpenKey(winreg.HKEY_CURRENT_USER, r'Software\Valve\Steam')
            path, _ = winreg.QueryValueEx(k, 'SteamPath')
            winreg.CloseKey(k)
            return path.replace('/', '\\')
        except Exception:
            pass
        for p in ['C:\\Program Files (x86)\\Steam', 'D:\\Steam', 'E:\\Steam']:
            if os.path.isdir(p):
                return p
        return None

    @staticmethod
    def _find_exes(folder, max_depth=2):
        exes = []
        skip = {'unins', 'unitycrashhandler', 'redist', 'vcredist',
                'dxwebsetup', 'dotnet', 'directx', 'crashreport',
                'installer', 'launcher', 'uninstall'}
        try:
            for root, dirs, files in os.walk(folder):
                depth = root[len(folder):].count(os.sep)
                if depth > max_depth:
                    continue
                for f in files:
                    if not f.lower().endswith('.exe'):
                        continue
                    fl = f.lower().replace('.exe', '')
                    if any(s in fl for s in skip):
                        continue
                    exes.append(os.path.join(root, f))
        except PermissionError:
            pass
        exes.sort(key=lambda x: os.path.getsize(x), reverse=True)
        return exes[:5]

    @staticmethod
    def scan_steam():
        games = []
        steam_path = GameScanner._get_steam_path()
        if not steam_path:
            return games
        lib_folders = [os.path.join(steam_path, 'steamapps')]
        vdf = os.path.join(steam_path, 'steamapps', 'libraryfolders.vdf')
        if os.path.exists(vdf):
            try:
                with open(vdf, 'r', encoding='utf-8') as f:
                    content = f.read()
                for m in re.finditer(r'"path"\s+"([^"]+)"', content):
                    p = m.group(1).replace('\\\\', '\\')
                    lib_folders.append(os.path.join(p, 'steamapps'))
            except Exception:
                pass
        for lib in lib_folders:
            common = os.path.join(lib, 'common')
            if not os.path.isdir(common):
                continue
            for d in os.listdir(common):
                full = os.path.join(common, d)
                if not os.path.isdir(full):
                    continue
                exes = GameScanner._find_exes(full)
                if exes:
                    games.append({
                        'name': d,
                        'process': os.path.basename(exes[0]),
                        'source': 'Steam'
                    })
        return games

    @staticmethod
    def scan_epic():
        games = []
        manifest_dir = os.path.join(
            os.environ.get('PROGRAMDATA', 'C:\\ProgramData'),
            'Epic', 'EpicGamesLauncher', 'Data', 'Manifests'
        )
        if not os.path.isdir(manifest_dir):
            return games
        for fname in os.listdir(manifest_dir):
            if not fname.endswith('.item'):
                continue
            fpath = os.path.join(manifest_dir, fname)
            try:
                with open(fpath, 'r', encoding='utf-8') as f:
                    content = f.read()
                nm = re.search(r'"DisplayName"\s*:\s*"([^"]+)"', content)
                pm = re.search(r'"InstallLocation"\s*:\s*"([^"]+)"', content)
                em = re.search(r'"LaunchExecutable"\s*:\s*"([^"]+)"', content)
                if nm and pm:
                    install = pm.group(1).replace('\\\\', '\\')
                    exe = em.group(1) if em else ''
                    if exe and not os.path.isabs(exe):
                        exe = os.path.join(install, exe)
                    if not exe or not os.path.exists(exe):
                        exes = GameScanner._find_exes(install)
                        exe = exes[0] if exes else ''
                    if exe:
                        games.append({
                            'name': nm.group(1),
                            'process': os.path.basename(exe),
                            'source': 'Epic'
                        })
            except Exception:
                continue
        return games

    @staticmethod
    def scan_all():
        all_g = []
        all_g.extend(GameScanner.scan_steam())
        all_g.extend(GameScanner.scan_epic())
        seen = set()
        unique = []
        for g in all_g:
            k = g['process'].lower()
            if k not in seen:
                seen.add(k)
                unique.append(g)
        return unique

# ========================================================
# Monitor thread
# ========================================================
class MonitorThread(threading.Thread):

    def __init__(self, config, on_status, on_log):
        super().__init__(daemon=True)
        self.config = config
        self.on_status = on_status
        self.on_log = on_log
        self.running = False
        self._toggled = False
        self._was_on = None
        self._was_gaming = False

    def stop(self):
        self.running = False

    def run(self):
        self.running = True
        poll = self.config['settings'].get('poll_interval_seconds', 5)
        self.on_log('info', '监控已启动')
        self.on_status('monitor', True)

        while self.running:
            try:
                enabled = [g for g in self.config['games'] if g.get('enabled', True)]
                running = [g for g in enabled if check_process(g['process'])]
                is_gaming = len(running) > 0

                if is_gaming and not self._was_gaming:
                    names = ', '.join(g['name'] for g in running)
                    self.on_log('game_start', '游戏启动: ' + names)
                    self._was_on = is_hdr_enabled()
                    auto_on = self.config['settings'].get('auto_hdr_on_game_start', True)
                    if auto_on and self._was_on is not True:
                        self.on_log('info', '检测到游戏，注册表HDR=' + ('开' if self._was_on else '关' if self._was_on is False else '未知') + '，自动开启HDR')
                        result = set_hdr_state(True)
                        self._toggled = True if result else False
                        self.on_log('success' if result else 'warning', 'HDR开启' + ('成功' if result else '可能失败'))
                    else:
                        self._toggled = False

                elif not is_gaming and self._was_gaming:
                    self.on_log('game_stop', '所有游戏已退出')
                    restore = self.config['settings'].get('restore_hdr_on_exit', True)
                    if restore and is_hdr_enabled() is True:
                        self.on_log('info', '正在关闭 HDR...')
                        result = set_hdr_state(False)
                        self._toggled = False
                        if result is False:
                            self.on_log('success', 'HDR 已关闭')
                        else:
                            self.on_log('warning', 'HDR 关闭可能失败')
                    elif self._was_on is True:
                        self.on_log('info', 'HDR 原本已开启，保持不变')
                    elif self._was_on is None:
                        self.on_log('info', 'HDR 状态未知，不做更改')

                self._was_gaming = is_gaming
                hdr_now = is_hdr_enabled()
                self.on_status('hdr', hdr_now)

            except Exception as e:
                self.on_log('error', '监控错误: ' + str(e))

            time.sleep(poll)

        self.on_log('info', '监控已停止')
        self.on_status('monitor', False)

# ========================================================
# ToggleSwitch widget
# ========================================================
class ToggleSwitch(tk.Canvas):

    def __init__(self, parent, command=None, width=44, height=24, **kw):
        self._tw = width
        self._th = height
        kw.setdefault('bg', COLORS['bg_panel'])
        super().__init__(parent, width=width, height=height,
                         highlightthickness=0, **kw)
        self.command = command
        self._on = False
        self.bind('<Button-1>', self._toggle)
        self._draw()

    def _draw(self):
        self.delete('all')
        pad = 2
        w, h = self._tw, self._th
        r = (h - 2 * pad) // 2
        if self._on:
            bg = COLORS['success']
            cx = w - pad - r
        else:
            bg = COLORS['scrollbar']
            cx = pad + r
        # rounded rect background
        pts = [pad+r, pad, w-pad-r, pad, w-pad, pad, w-pad, pad+r,
               w-pad, h-pad-r, w-pad, h-pad, w-pad-r, h-pad,
               pad+r, h-pad, pad, h-pad, pad, h-pad-r, pad, pad+r]
        self.create_polygon(pts, fill=bg, outline='')
        # thumb
        sr = r - 2
        self.create_oval(cx-sr, pad+2, cx+sr, h-pad-2,
                         fill='#ffffff', outline='')

    def _toggle(self, event=None):
        self._on = not self._on
        self._draw()
        if self.command:
            self.command(self._on)

    def set(self, value):
        self._on = bool(value)
        self._draw()

    def get(self):
        return self._on

# ========================================================
# Scrollable frame
# ========================================================
class ScrollableFrame(ttk.Frame):

    def __init__(self, parent, **kw):
        super().__init__(parent, **kw)
        self.canvas = tk.Canvas(self, bg=COLORS['bg_panel'],
                                highlightthickness=0, bd=0)
        self.scrollbar = ttk.Scrollbar(self, orient='vertical',
                                        command=self.canvas.yview)
        self.inner = ttk.Frame(self.canvas, style='Dark.TFrame')
        self.inner_id = self.canvas.create_window(
            (0, 0), window=self.inner, anchor='nw', tags='inner')
        self.canvas.configure(yscrollcommand=self.scrollbar.set)
        self.canvas.pack(side='left', fill='both', expand=True)
        self.scrollbar.pack(side='right', fill='y')
        self.inner.bind('<Configure>',
                        lambda e: self.canvas.configure(
                            scrollregion=self.canvas.bbox('all')))
        self.canvas.bind('<Configure>',
                         lambda e: self.canvas.itemconfig(
                             self.inner_id, width=e.width))
        self.canvas.bind('<Enter>', lambda e: self.canvas.bind_all(
            '<MouseWheel>', lambda ev: self.canvas.yview_scroll(
                int(-1*(ev.delta/120)), 'units')))
        self.canvas.bind('<Leave>',
                         lambda e: self.canvas.unbind_all('<MouseWheel>'))

# ========================================================
# Main app
# ========================================================
class GameHDRApp:

    def __init__(self):
        self.root = tk.Tk()
        self.root.title('游戏 HDR 管理器')
        self.root.geometry('880x640')
        self.root.minsize(750, 500)
        self.root.configure(bg=COLORS['bg_dark'])
        self.config = load_config()
        self.monitor = None
        self.scanned_games = []
        self.game_rows = []
        self._setup_styles()
        self._build_ui()
        self._load_games()
        # Clean up stale registry value from previous buggy runs
        if is_hdr_enabled() is True:
            _write_hdr(False)
        self._refresh_hdr()
        self._poll_hdr()
        self._check_autostart()
        self.root.protocol('WM_DELETE_WINDOW', self._on_close)

    # ---- styles ----
    def _setup_styles(self):
        style = ttk.Style(self.root)
        style.theme_use('clam')
        style.configure('.', font=FONTS['body'],
                        background=COLORS['bg_dark'],
                        foreground=COLORS['text'], borderwidth=0)
        style.configure('Dark.TFrame', background=COLORS['bg_dark'])
        style.configure('Panel.TFrame', background=COLORS['bg_panel'])
        style.configure('Card.TFrame', background=COLORS['bg_card'])
        style.configure('Dark.TLabel', background='',
                        foreground=COLORS['text'], font=FONTS['body'])
        style.configure('Title.TLabel', font=FONTS['title'],
                        foreground=COLORS['text'], background=COLORS['bg_dark'])
        style.configure('Heading.TLabel', font=FONTS['heading'],
                        foreground=COLORS['text'], background='')
        style.configure('Dim.TLabel', font=FONTS['small'],
                        foreground=COLORS['text_dim'], background='')
        style.configure('Success.TLabel', font=FONTS['body'],
                        foreground=COLORS['success'], background='')
        style.configure('Warning.TLabel', font=FONTS['body'],
                        foreground=COLORS['warning'], background='')
        style.configure('Dark.TButton', font=FONTS['body'],
                        background=COLORS['bg_card'], foreground=COLORS['text'],
                        borderwidth=1, relief='flat', padding=(12, 6))
        style.map('Dark.TButton',
                  background=[('active', COLORS['bg_hover']),
                              ('pressed', COLORS['bg_input'])],
                  foreground=[('active', COLORS['accent_hover'])])
        style.configure('Accent.TButton', font=FONTS['body'],
                        background=COLORS['accent'], foreground='#ffffff',
                        borderwidth=0, relief='flat', padding=(14, 7))
        style.map('Accent.TButton',
                  background=[('active', COLORS['accent_hover'])])
        style.configure('SuccessBtn.TButton', font=FONTS['body'],
                        background=COLORS['success_dim'], foreground='#ffffff',
                        borderwidth=0, relief='flat', padding=(14, 7))
        style.map('SuccessBtn.TButton',
                  background=[('active', COLORS['success'])])
        style.configure('Danger.TButton', font=FONTS['body'],
                        background=COLORS['danger'], foreground='#ffffff',
                        borderwidth=0, relief='flat', padding=(14, 7))
        style.configure('Dark.TEntry', fieldbackground=COLORS['bg_input'],
                        foreground=COLORS['text'], insertcolor=COLORS['text'],
                        borderwidth=1, relief='flat', padding=6)
        style.configure('TScrollbar', background=COLORS['bg_panel'],
                        troughcolor=COLORS['bg_dark'],
                        arrowcolor=COLORS['text_dim'], borderwidth=0)

    # ---- build UI ----
    def _build_ui(self):
        # title bar
        bar = tk.Frame(self.root, bg=COLORS['bg_panel'], height=52)
        bar.pack(fill='x')
        bar.pack_propagate(False)

        tf = tk.Frame(bar, bg=COLORS['bg_panel'])
        tf.pack(side='left', padx=(16,0), pady=4)
        tk.Label(tf, text='游戏 HDR 管理器', font=FONTS['title'],
                 bg=COLORS['bg_panel'], fg=COLORS['text']).pack(side='left')

        right = tk.Frame(bar, bg=COLORS['bg_panel'])
        right.pack(side='right', padx=16, pady=4)

        self.hdr_cv = tk.Canvas(right, width=36, height=36,
                                highlightthickness=0, bg=COLORS['bg_panel'])
        self.hdr_cv.pack(side='right', padx=(4,0))
        self._dot(self.hdr_cv, 'unknown')

        tk.Label(right, text='HDR', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['text_dim']).pack(
                     side='right', padx=(0,4))

        self.mon_cv = tk.Canvas(right, width=36, height=36,
                                highlightthickness=0, bg=COLORS['bg_panel'])
        self.mon_cv.pack(side='right', padx=(4,0))
        self._dot(self.mon_cv, False)

        tk.Label(right, text='监控', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['text_dim']).pack(
                     side='right', padx=(0,4))

        tk.Label(right, text='|', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['border']).pack(
                     side='right', padx=(8,8))

        self.count_lbl = tk.Label(right, text='Games: 0', font=FONTS['small'],
                                  bg=COLORS['bg_panel'], fg=COLORS['text_dim'])
        self.count_lbl.pack(side='right')

        # main body
        main = ttk.Frame(self.root, style='Dark.TFrame')
        main.pack(fill='both', expand=True, padx=2, pady=(0,2))

        left = ttk.Frame(main, style='Panel.TFrame')
        left.place(relx=0, rely=0, relwidth=0.58, relheight=1.0)
        self._build_game_list(left)

        rp = ttk.Frame(main, style='Panel.TFrame')
        rp.place(relx=0.58, rely=0, relwidth=0.42, relheight=1.0, x=2)
        self._build_control_panel(rp)

        self._build_log_panel()

    def _build_game_list(self, parent):
        hdr = tk.Frame(parent, bg=COLORS['bg_panel'], height=40)
        hdr.pack(fill='x', padx=12, pady=(12,0))
        hdr.pack_propagate(False)
        tk.Label(hdr, text='游戏列表', font=FONTS['heading'],
                 bg=COLORS['bg_panel'], fg=COLORS['text']).pack(side='left')

        bf = tk.Frame(hdr, bg=COLORS['bg_panel'])
        bf.pack(side='right')

        tk.Button(bf, text='+ 手动添加', font=FONTS['small'],
                  bg=COLORS['bg_card'], fg=COLORS['accent'],
                  activebackground=COLORS['bg_hover'],
                  activeforeground=COLORS['accent_hover'],
                  bd=0, padx=10, pady=3, cursor='hand2',
                  command=self._manual_add).pack(side='left', padx=(0,6))

        tk.Button(bf, text='扫描游戏', font=FONTS['small'],
                  bg=COLORS['accent'], fg='#ffffff',
                  activebackground=COLORS['accent_hover'],
                  activeforeground='#ffffff',
                  bd=0, padx=10, pady=3, cursor='hand2',
                  command=self._scan_games).pack(side='left')

        # column headers
        ch = tk.Frame(parent, bg=COLORS['bg_panel'], height=28)
        ch.pack(fill='x', padx=12, pady=(6,0))
        ch.pack_propagate(False)
        tk.Label(ch, text='名称', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['text_muted'],
                 anchor='w').pack(side='left', padx=(8,0))
        tk.Label(ch, text='进程', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['text_muted'],
                 width=22, anchor='w').pack(side='right', padx=(0,60))
        tk.Label(ch, text='启用', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['text_muted'],
                 width=5, anchor='c').pack(side='right', padx=(0,8))

        self.game_list_frame = ScrollableFrame(parent)
        self.game_list_frame.pack(fill='both', expand=True, padx=8, pady=4)

    def _build_control_panel(self, parent):
        # Monitor control
        for (header, section_id) in [
            ('监控控制', 'monitor'),
            ('切换 HDR', 'hdr'),
            ('设置', 'settings'),
        ]:
            card = tk.Frame(parent, bg=COLORS['bg_card'],
                            highlightthickness=1,
                            highlightbackground=COLORS['border'])
            card.pack(fill='x', padx=12, pady=(12 if section_id == 'monitor' else 0, 8))

            ch = tk.Frame(card, bg=COLORS['bg_card'], height=36)
            ch.pack(fill='x', padx=12, pady=(8,0))
            ch.pack_propagate(False)
            tk.Label(ch, text=header, font=FONTS['heading'],
                     bg=COLORS['bg_card'], fg=COLORS['text']).pack(side='left')

            inner = tk.Frame(card, bg=COLORS['bg_card'])
            inner.pack(fill='x', padx=12, pady=(8,12))

            if section_id == 'monitor':
                self.mon_btn = tk.Button(
                    inner, text='启动监控', font=FONTS['body'],
                    bg=COLORS['success_dim'], fg='#ffffff',
                    activebackground=COLORS['success'],
                    activeforeground='#ffffff',
                    bd=0, padx=16, pady=8, cursor='hand2',
                    command=self._toggle_monitor)
                self.mon_btn.pack(fill='x')
                tk.Label(inner, text='游戏运行时自动切换 HDR',
                         font=FONTS['small'], bg=COLORS['bg_card'],
                         fg=COLORS['text_dim']).pack(pady=(4,0))

            elif section_id == 'hdr':
                tk.Button(inner, text='切换 HDR (Win+Alt+B)',
                          font=FONTS['body'], bg=COLORS['accent'],
                          fg='#ffffff',
                          activebackground=COLORS['accent_hover'],
                          activeforeground='#ffffff',
                          bd=0, padx=16, pady=10, cursor='hand2',
                          command=self._manual_toggle_hdr).pack(fill='x')
                self.hdr_status = tk.Label(
                    inner, text='检测中...', font=FONTS['small'],
                    bg=COLORS['bg_card'], fg=COLORS['text_dim'])
                self.hdr_status.pack(pady=(6,0))

            elif section_id == 'settings':
                for (label, initial, callback) in [
                    ('退出时恢复 HDR',
                     self.config['settings'].get('restore_hdr_on_exit', True),
                     self._on_restore_toggle),
                    ('启动时自动监控',
                     self.config['settings'].get('start_monitoring_on_launch', False),
                     self._on_autostart_toggle),
                    ('游戏启动时自动开启HDR',
                     self.config['settings'].get('auto_hdr_on_game_start', True),
                     self._on_auto_hdr_toggle),
                ]:
                    row = tk.Frame(inner, bg=COLORS['bg_card'])
                    row.pack(fill='x', pady=(0,6))
                    tk.Label(row, text=label, font=FONTS['body'],
                             bg=COLORS['bg_card'],
                             fg=COLORS['text']).pack(side='left')
                    ts = ToggleSwitch(row, command=callback, bg=COLORS['bg_card'])
                    ts.set(initial)
                    ts.pack(side='right')
                    if callback == self._on_restore_toggle:
                        self.restore_ts = ts
                    elif callback == self._on_autostart_toggle:
                        self.autostart_ts = ts
                    elif callback == self._on_auto_hdr_toggle:
                        self.auto_hdr_ts = ts

                # Poll interval
                row3 = tk.Frame(inner, bg=COLORS['bg_card'])
                row3.pack(fill='x')
                tk.Label(row3, text='轮询间隔(秒)', font=FONTS['body'],
                         bg=COLORS['bg_card'],
                         fg=COLORS['text']).pack(side='left')
                self.poll_var = tk.StringVar(
                    value=str(self.config['settings'].get(
                        'poll_interval_seconds', 5)))
                sf = tk.Frame(row3, bg=COLORS['bg_card'])
                sf.pack(side='right')
                pe = tk.Entry(sf, textvariable=self.poll_var,
                              width=4, font=FONTS['body'],
                              bg=COLORS['bg_input'], fg=COLORS['text'],
                              insertbackground=COLORS['text'],
                              relief='flat', bd=1, justify='center')
                pe.pack(side='left')
                sb = tk.Frame(sf, bg=COLORS['bg_card'])
                sb.pack(side='left', padx=(2,0))
                def _up():
                    try:
                        v = int(self.poll_var.get()) + 1
                        if v <= 60:
                            self.poll_var.set(str(v))
                            self._on_poll_change()
                    except ValueError:
                        pass
                def _down():
                    try:
                        v = int(self.poll_var.get()) - 1
                        if v >= 1:
                            self.poll_var.set(str(v))
                            self._on_poll_change()
                    except ValueError:
                        pass
                for (t, c) in [('+', _up), ('-', _down)]:
                    tk.Button(sb, text=t, font=('Segoe UI', 6),
                              bg=COLORS['bg_card'], fg=COLORS['text_dim'],
                              activebackground=COLORS['bg_hover'],
                              activeforeground=COLORS['accent'],
                              bd=0, width=2, height=1,
                              cursor='hand2', command=c).pack()
                pe.bind('<FocusOut>', lambda e: self._on_poll_change())

    def _build_log_panel(self):
        lf = tk.Frame(self.root, bg=COLORS['bg_panel'], height=140)
        lf.pack(fill='x', side='bottom')
        lf.pack_propagate(False)

        lh = tk.Frame(lf, bg=COLORS['bg_panel'], height=28)
        lh.pack(fill='x', padx=12, pady=(4,0))
        lh.pack_propagate(False)
        tk.Label(lh, text='日志', font=FONTS['small'],
                 bg=COLORS['bg_panel'], fg=COLORS['text_dim']).pack(side='left')
        bf = tk.Frame(lh, bg=COLORS['bg_panel'])
        bf.pack(side='right')
        tk.Button(bf, text='清除', font=FONTS['small'],
                  bg=COLORS['bg_card'], fg=COLORS['text_dim'],
                  activebackground=COLORS['bg_hover'],
                  bd=0, padx=8, pady=1, cursor='hand2',
                  command=self._clear_log).pack(side='left', padx=(0,6))
        tk.Button(bf, text='导出', font=FONTS['small'],
                  bg=COLORS['bg_card'], fg=COLORS['text_dim'],
                  activebackground=COLORS['bg_hover'],
                  bd=0, padx=8, pady=1, cursor='hand2',
                  command=self._export_log).pack(side='left')

        tf = tk.Frame(lf, bg=COLORS['bg_input'],
                      highlightthickness=1,
                      highlightbackground=COLORS['border'])
        tf.pack(fill='both', expand=True, padx=8, pady=(2,8))

        self.log_text = tk.Text(tf, font=FONTS['mono_sm'],
                                bg=COLORS['bg_input'], fg=COLORS['text'],
                                insertbackground=COLORS['text'],
                                wrap='word', bd=0, padx=8, pady=4,
                                state='disabled', height=5)
        self.log_text.pack(fill='both', expand=True)
        for tag, color in [('info', COLORS['text_dim']),
                           ('success', COLORS['success']),
                           ('warning', COLORS['warning']),
                           ('error', COLORS['danger']),
                           ('game_start', COLORS['cyan']),
                           ('game_stop', COLORS['accent']),
                           ('timestamp', COLORS['text_muted'])]:
            self.log_text.tag_config(tag, foreground=color)

    # ---- game rows ----
    def _load_games(self):
        for row in self.game_rows:
            for w in row:
                w.destroy()
        self.game_rows.clear()
        for g in self.config.get('games', []):
            self._add_row(g)
        self._update_count()

    def _add_row(self, game):
        inner = self.game_list_frame.inner
        bg = COLORS['bg_card'] if len(self.game_rows) % 2 == 0 else COLORS['bg_panel']
        rf = tk.Frame(inner, bg=bg, height=38)
        rf.pack(fill='x', pady=1)
        rf.pack_propagate(False)

        nf = tk.Frame(rf, bg=bg)
        nf.pack(side='left', padx=(8,0), fill='x', expand=True)

        src = game.get('source', '')
        if src:
            sc = {'Steam':'#1a5fb4', 'Epic':'#9147ff'}.get(src, COLORS['text_muted'])
            tk.Label(nf, text=src, font=('Segoe UI',7,'bold'),
                     bg=sc, fg='#ffffff', padx=4, pady=1).pack(
                         side='left', padx=(0,6))

        tk.Label(nf, text=game['name'], font=FONTS['body'],
                 bg=bg, fg=COLORS['text'],
                 anchor='w').pack(side='left')

        tk.Label(rf, text=game['process'], font=FONTS['mono_sm'],
                 bg=bg, fg=COLORS['text_dim'], width=22,
                 anchor='w').pack(side='right', padx=(0,8))

        lb = tk.Label(rf, text='启动', font=('Segoe UI',9,'bold'),
                      bg=COLORS['success_dim'], fg='#ffffff', cursor='hand2',
                      padx=6, pady=1)
        lb.pack(side='right', padx=(0,6))
        lb.bind('<Button-1>', lambda e, g=game: self._launch_game(g))
        lb.bind('<Enter>', lambda e: lb.configure(bg=COLORS['success']))
        lb.bind('<Leave>', lambda e: lb.configure(bg=COLORS['success_dim']))

        ts = ToggleSwitch(
            rf,
            command=lambda v, g=game: self._on_game_toggle(g, v),
            bg=bg)
        ts.set(game.get('enabled', True))
        ts.pack(side='right', padx=(0,12))

        dl = tk.Label(rf, text='x', font=FONTS['small'],
                      bg=bg, fg=COLORS['text_muted'], cursor='hand2')
        dl.pack(side='right', padx=(0,4))
        dl.bind('<Button-1>', lambda e, g=game: self._delete_game(g))
        dl.bind('<Enter>', lambda e: dl.configure(fg=COLORS['danger']))
        dl.bind('<Leave>', lambda e: dl.configure(fg=COLORS['text_muted']))

        self.game_rows.append([rf, ts, dl])

    def _update_count(self):
        e = sum(1 for g in self.config.get('games',[]) if g.get('enabled',True))
        t = len(self.config.get('games',[]))
        self.count_lbl.configure(text='游戏: ' + str(e) + '/' + str(t))

    def _on_game_toggle(self, game, value):
        game['enabled'] = value
        save_config(self.config)
        self._update_count()
        self._log(('已启用: ' if value else '已禁用: ') + game['name'], 'info')

    def _launch_game(self, game):
        """Toggle HDR ON, then launch the game. If no path, ask user to locate exe."""
        nm = game['name']
        path = game.get('path', '')
        if not path or not os.path.exists(path):
            path = filedialog.askopenfilename(
                title='定位 ' + nm + ' 的可执行文件',
                filetypes=[('Executable', '*.exe'), ('All files', '*.*')])
            if not path:
                return
            game['path'] = path
            save_config(self.config)
            self._log('已设置路径: ' + path, 'info')
        self._log('正在开启HDR并启动: ' + nm, 'info')
        set_hdr_state(True, force=True)
        time.sleep(1.0)
        try:
            os.startfile(path)
            self._log('已启动: ' + nm, 'success')
        except Exception as e:
            self._log('启动失败 ' + nm + ': ' + str(e), 'error')

    def _delete_game(self, game):
        nm = game['name']
        self.config['games'] = [
            g for g in self.config['games']
            if g['process'] != game['process']]
        save_config(self.config)
        self._load_games()
        self._log('已删除: ' + nm, 'info')

    def _manual_add(self):
        dlg = tk.Toplevel(self.root)
        dlg.title('添加游戏')
        dlg.geometry('400x220')
        dlg.configure(bg=COLORS['bg_panel'])
        dlg.resizable(False, False)
        dlg.transient(self.root)
        dlg.grab_set()
        self._center_window(dlg, 400, 220)

        tk.Label(dlg, text='添加游戏', font=FONTS['heading'],
                 bg=COLORS['bg_panel'],
                 fg=COLORS['text']).pack(pady=(16,12))

        for (label, row_var) in [('游戏名称', 'name'), ('进程(.exe)', 'proc')]:
            f = tk.Frame(dlg, bg=COLORS['bg_panel'])
            f.pack(fill='x', padx=24, pady=(0,8))
            tk.Label(f, text=label, font=FONTS['body'],
                     bg=COLORS['bg_panel'], fg=COLORS['text_dim'],
                     width=12, anchor='w').pack(side='left')
            e = tk.Entry(f, font=FONTS['body'],
                         bg=COLORS['bg_input'], fg=COLORS['text'],
                         insertbackground=COLORS['text'],
                         relief='flat', bd=1)
            e.pack(side='right', fill='x', expand=True)
            if row_var == 'name':
                self._dlg_name = e
            else:
                self._dlg_proc = e

        tk.Label(dlg, text='在任务管理器 > 详细信息中查找进程名',
                 font=FONTS['small'], bg=COLORS['bg_panel'],
                 fg=COLORS['text_muted']).pack()

        def _confirm():
            nm = self._dlg_name.get().strip()
            pr = self._dlg_proc.get().strip()
            if not nm or not pr:
                messagebox.showwarning('提示', '请填写所有字段')
                return
            if not pr.lower().endswith('.exe'):
                pr += '.exe'
            if any(g['process'].lower() == pr.lower()
                   for g in self.config['games']):
                messagebox.showwarning('提示', '进程已存在: ' + pr)
                return
            self.config['games'].append({
                'name': nm, 'process': pr,
                'enabled': True, 'source': 'Manual'
            })
            save_config(self.config)
            self._load_games()
            self._log('已添加: ' + nm + ' (' + pr + ')', 'success')
            dlg.destroy()

        bf = tk.Frame(dlg, bg=COLORS['bg_panel'])
        bf.pack(fill='x', padx=24, pady=(4,0))
        tk.Button(bf, text='取消', font=FONTS['body'],
                  bg=COLORS['bg_card'], fg=COLORS['text_dim'],
                  activebackground=COLORS['bg_hover'],
                  bd=0, padx=16, pady=4, cursor='hand2',
                  command=dlg.destroy).pack(side='left')
        tk.Button(bf, text='添加', font=FONTS['body'],
                  bg=COLORS['accent'], fg='#ffffff',
                  activebackground=COLORS['accent_hover'],
                  bd=0, padx=20, pady=4, cursor='hand2',
                  command=_confirm).pack(side='right')

    def _scan_games(self):
        self._log('正在扫描已安装的游戏...', 'info')
        self.scanned_games = GameScanner.scan_all()
        if not self.scanned_games:
            messagebox.showinfo('扫描', '未找到游戏。')
            self._log('扫描完成：未发现游戏', 'warning')
            return

        dlg = tk.Toplevel(self.root)
        dlg.title('扫描结果')
        dlg.geometry('520x400')
        dlg.configure(bg=COLORS['bg_panel'])
        dlg.transient(self.root)
        dlg.grab_set()
        self._center_window(dlg, 520, 400)

        tk.Label(dlg,
                 text='发现 ' + str(len(self.scanned_games)) + ' 个游戏',
                 font=FONTS['heading'], bg=COLORS['bg_panel'],
                 fg=COLORS['text']).pack(pady=(14,4))
        tk.Label(dlg, text='选择要添加到监控列表的游戏',
                 font=FONTS['small'], bg=COLORS['bg_panel'],
                 fg=COLORS['text_dim']).pack()

        lf = tk.Frame(dlg, bg=COLORS['bg_panel'])
        lf.pack(fill='both', expand=True, padx=14, pady=8)

        cv = tk.Canvas(lf, bg=COLORS['bg_panel'], highlightthickness=0)
        sb = ttk.Scrollbar(lf, orient='vertical', command=cv.yview)
        inner = tk.Frame(cv, bg=COLORS['bg_panel'])
        iid = cv.create_window((0,0), window=inner, anchor='nw')
        cv.configure(yscrollcommand=sb.set)
        cv.pack(side='left', fill='both', expand=True)
        sb.pack(side='right', fill='y')
        cv.bind('<Configure>', lambda e: cv.itemconfig(iid, width=e.width))
        inner.bind('<Configure>',
                   lambda e: cv.configure(scrollregion=cv.bbox('all')))

        check_vars = {}
        existing = {g['process'].lower() for g in self.config['games']}

        for i, game in enumerate(self.scanned_games):
            bg = COLORS['bg_card'] if i % 2 == 0 else COLORS['bg_panel']
            row = tk.Frame(inner, bg=bg, height=36)
            row.pack(fill='x', pady=1)
            row.pack_propagate(False)

            already = game['process'].lower() in existing
            var = tk.BooleanVar(value=not already)
            check_vars[game['process']] = var
            st = 'disabled' if already else 'normal'

            tk.Checkbutton(row, variable=var, bg=bg, fg=COLORS['text'],
                           selectcolor=COLORS['bg_input'],
                           activebackground=bg,
                           activeforeground=COLORS['accent'],
                           font=FONTS['body'], state=st,
                           text='').pack(side='left', padx=(6,4))

            sc = {'Steam':'#1a5fb4', 'Epic':'#9147ff'}.get(
                game['source'], COLORS['text_muted'])
            tk.Label(row, text=game['source'],
                     font=('Segoe UI',7,'bold'),
                     bg=sc, fg='#ffffff', padx=4,
                     pady=1).pack(side='left', padx=(0,6))

            tk.Label(row, text=game['name'], font=FONTS['body'],
                     bg=bg,
                     fg=COLORS['text'] if not already else COLORS['text_muted'],
                     anchor='w').pack(side='left')

            tk.Label(row, text='已添加' if already else '',
                     font=FONTS['small'], bg=bg,
                     fg=COLORS['success'] if already else COLORS['text_muted']
                     ).pack(side='right', padx=(0,12))
            tk.Label(row, text=game['process'], font=FONTS['mono_sm'],
                     bg=bg, fg=COLORS['text_dim']).pack(
                         side='right', padx=(0,12))

        def _add_selected():
            added = 0
            for game in self.scanned_games:
                if check_vars[game['process']].get():
                    if game['process'].lower() not in existing:
                        self.config['games'].append({
                            'name': game['name'],
                            'process': game['process'],
                            'enabled': True,
                            'source': game['source']
                        })
                        added += 1
            if added > 0:
                save_config(self.config)
                self._load_games()
                self._log('已添加 ' + str(added) + ' 个游戏(扫描)', 'success')
            dlg.destroy()

        bf = tk.Frame(dlg, bg=COLORS['bg_panel'])
        bf.pack(fill='x', padx=14, pady=(4,14))
        tk.Button(bf, text='取消', font=FONTS['body'],
                  bg=COLORS['bg_card'], fg=COLORS['text_dim'],
                  activebackground=COLORS['bg_hover'],
                  bd=0, padx=16, pady=5, cursor='hand2',
                  command=dlg.destroy).pack(side='left')
        tk.Button(bf, text='添加选中', font=FONTS['body'],
                  bg=COLORS['accent'], fg='#ffffff',
                  activebackground=COLORS['accent_hover'],
                  bd=0, padx=20, pady=5, cursor='hand2',
                  command=_add_selected).pack(side='right')

    # ---- monitor ----
    def _toggle_monitor(self):
        if self.monitor and self.monitor.running:
            self._stop_monitor()
        else:
            self._start_monitor()

    def _start_monitor(self):
        enabled = [g for g in self.config['games'] if g.get('enabled', True)]
        if not enabled:
            messagebox.showinfo('提示', '没有启用任何游戏，请先添加并启用游戏。')
            return
        self.monitor = MonitorThread(self.config, self._on_status, self._log)
        self.monitor.start()
        self.mon_btn.configure(text='停止监控', bg=COLORS['danger'],
                               activebackground='#ff6b6b')
        self._log('监控已启动', 'success')

    def _stop_monitor(self):
        if self.monitor:
            self.monitor.stop()
            self.monitor = None
        self.mon_btn.configure(text='启动监控', bg=COLORS['success_dim'],
                               activebackground=COLORS['success'])
        self._on_status('monitor', False)
        self._log('监控已停止', 'info')

    def _on_status(self, st, val):
        self.root.after(0, lambda: self._update_status(st, val))

    def _update_status(self, st, val):
        if st == 'hdr':
            if val is True:
                dd = 'on'
                self.hdr_status.configure(text='HDR: 开', fg=COLORS['success'])
            elif val is False:
                dd = 'off'
                self.hdr_status.configure(text='HDR: 关', fg=COLORS['text_dim'])
            else:
                dd = 'unknown'
                self.hdr_status.configure(text='HDR: ?', fg=COLORS['warning'])
            self._dot(self.hdr_cv, dd)
        elif st == 'monitor':
            self._dot(self.mon_cv, val)

    def _refresh_hdr(self):
        self._update_status('hdr', is_hdr_enabled())

    def _poll_hdr(self):
        self._refresh_hdr()
        self.root.after(3000, self._poll_hdr)

    def _check_autostart(self):
        if self.config['settings'].get('start_monitoring_on_launch', False):
            self.root.after(1000, self._start_monitor)

    # ---- manual HDR ----
    def _manual_toggle_hdr(self):
        self._log('手动切换 HDR...', 'info')
        send_win_alt_b()
        time.sleep(0.5)
        hdr = is_hdr_enabled()
        if hdr is True:
            st = 'ON'
        elif hdr is False:
            st = 'OFF'
        else:
            st = '?'
        self._log('手动切换完成，HDR: ' + st, 'info')
        self._refresh_hdr()

    # ---- settings ----
    def _on_restore_toggle(self, val):
        self.config['settings']['restore_hdr_on_exit'] = val
        save_config(self.config)

    def _on_autostart_toggle(self, val):
        self.config['settings']['start_monitoring_on_launch'] = val
        save_config(self.config)

    def _on_auto_hdr_toggle(self, val):
        self.config['settings']['auto_hdr_on_game_start'] = val
        save_config(self.config)

    def _on_poll_change(self):
        self.config['settings']['start_monitoring_on_launch'] = val
        save_config(self.config)

    def _on_poll_change(self):
        try:
            v = int(self.poll_var.get())
            if 1 <= v <= 60:
                self.config['settings']['poll_interval_seconds'] = v
                save_config(self.config)
        except ValueError:
            pass

    # ---- status dots ----
    def _dot(self, cv, state):
        cv.delete('all')
        cx, cy, r = 18, 18, 12
        if state == 'on':
            color, glow = COLORS['hdr_on'], '#3fb950'
        elif state == 'off':
            color, glow = COLORS['hdr_off'], '#30363d'
        elif state is True:
            color, glow = COLORS['success'], '#3fb950'
            cv.create_oval(cx-r-1, cy-r-1, cx+r+1, cy+r+1,
                           fill='', outline=COLORS['success'], width=2)
        elif state is False:
            color, glow = COLORS['hdr_off'], '#30363d'
        else:
            color, glow = COLORS['warning'], '#d29922'
        cv.create_oval(cx-r-3, cy-r-3, cx+r+3, cy+r+3,
                       fill='', outline=glow, width=2)
        cv.create_oval(cx-r, cy-r, cx+r, cy+r, fill=color, outline='')

    # ---- logging ----
    def _log(self, msg, level='info'):
        self.root.after(0, lambda: self._wlog(msg, level))

    def _wlog(self, msg, level):
        ts = datetime.now().strftime('%H:%M:%S')
        self.log_text.configure(state='normal')
        self.log_text.insert('end', ts + ' ', 'timestamp')
        tag = {'info':'[信息]','success':'[成功]','warning':'[警告]','error':'[错误]','game_start':'[游戏启动]','game_stop':'[游戏退出]'}.get(level,'['+level+']')
        self.log_text.insert('end', tag + ' ' + msg + '\n', level)
        self.log_text.see('end')
        self.log_text.configure(state='disabled')

    def _clear_log(self):
        self.log_text.configure(state='normal')
        self.log_text.delete('1.0', 'end')
        self.log_text.configure(state='disabled')

    def _export_log(self):
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        fn = filedialog.asksaveasfilename(
            defaultextension=".txt",
            filetypes=[("Text files", "*.txt"), ("All files", "*.*")],
            initialfile="hdr_log_" + ts + ".txt"
        )
        if fn:
            with open(fn, 'w', encoding='utf-8') as f:
                f.write(self.log_text.get('1.0', 'end-1c'))
            self._log('日志已导出: ' + fn, 'success')

    # ---- window ----
    def _on_close(self):
        if self.monitor and self.monitor.running:
            if messagebox.askyesno('确认', '监控正在运行，确定退出吗？'):
                self._stop_monitor()
            else:
                return
        self.root.destroy()

    def _center_window(self, w, ww, wh):
        w.update_idletasks()
        x = self.root.winfo_x() + (self.root.winfo_width() - ww) // 2
        y = self.root.winfo_y() + (self.root.winfo_height() - wh) // 2
        w.geometry(str(ww) + 'x' + str(wh) + '+' + str(x) + '+' + str(y))

    def run(self):
        self.root.mainloop()

# ========================================================
def main():
    def _show_err(et, ev, tb):
        import traceback
        traceback.print_exception(et, ev, tb)
        try:
            messagebox.showerror('错误',
                                 '未预期的错误:\n\n' + str(ev))
        except Exception:
            pass

    sys.excepthook = _show_err

    if sys.platform != 'win32':
        messagebox.showerror('错误', '仅支持 Windows。')
        sys.exit(1)

    try:
        import tkinter
        tkinter.Tk().destroy()
    except Exception as e:
        messagebox.showerror('错误', 'tkinter 不可用: ' + str(e))
        sys.exit(1)

    try:
        app = GameHDRApp()
        app.run()
    except Exception as e:
        import traceback
        traceback.print_exc()
        messagebox.showerror('启动错误', str(e))
        sys.exit(1)

if __name__ == '__main__':
    main()
