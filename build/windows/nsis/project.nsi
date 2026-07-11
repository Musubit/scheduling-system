Unicode true

!include "wails_tools.nsh"

!define PRODUCT_NAME "SchedulingSystem"
!define APP_EXECUTABLE "scheduling-system.exe"

Name "${PRODUCT_NAME}"
OutFile "${PRODUCT_NAME}-installer.exe"
InstallDir "$PROGRAMFILES\scheduling-system"
RequestExecutionLevel admin

Section "Install"
  SetOutPath "$INSTDIR"
  File "MicrosoftEdgeWebview2Setup.exe"
  ExecWait '"$INSTDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install'
  File "${ARG_WAILS_AMD64_BINARY}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\${APP_EXECUTABLE}"
  CreateShortCut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${APP_EXECUTABLE}"
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\${APP_EXECUTABLE}"
  Delete "$INSTDIR\MicrosoftEdgeWebview2Setup.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  RMDir "$SMPROGRAMS\${PRODUCT_NAME}"
  Delete "$DESKTOP\${PRODUCT_NAME}.lnk"
SectionEnd
