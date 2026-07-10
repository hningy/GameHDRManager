Option Explicit

Dim shell, base, exe
Set shell = CreateObject("WScript.Shell")
base = Left(WScript.ScriptFullName, InStrRev(WScript.ScriptFullName, "\"))
exe = base & "bin\GameHDRManager.exe"
shell.Run """" & exe & """", 0, False
