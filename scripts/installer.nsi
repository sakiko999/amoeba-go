Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma

!ifndef OUT_FILE
!error "OUT_FILE is required"
!endif

Name "amoeba-go"
OutFile "${OUT_FILE}"
!define APP_EXE "$%APP_EXE_ENV%"
!define APP_VERSION "$%APP_VERSION_ENV%"
InstallDir "$LOCALAPPDATA\Programs\amoeba-go"
InstallDirRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\amoeba-go" "InstallLocation"
RequestExecutionLevel user
BrandingText " "

!define MUI_FINISHPAGE_RUN "$INSTDIR\amoeba-go.exe"
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "创建桌面快捷方式"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!include "MUI2.nsh"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\amoeba-go"

Section "amoeba-go"
  SetOutPath "$INSTDIR"
  File "${APP_EXE}"
  Rename "$INSTDIR\mygo-host.exe" "$INSTDIR\amoeba-go.exe"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortCut "$SMPROGRAMS\amoeba-go.lnk" "$INSTDIR\amoeba-go.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "amoeba-go"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\amoeba-go.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Function CreateDesktopShortcut
  SetOutPath "$INSTDIR"
  CreateShortCut "$DESKTOP\amoeba-go.lnk" "$INSTDIR\amoeba-go.exe"
FunctionEnd

Section "Uninstall"
  Delete "$SMPROGRAMS\amoeba-go.lnk"
  Delete "$DESKTOP\amoeba-go.lnk"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  RMDir /r "$INSTDIR"
SectionEnd
