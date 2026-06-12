!include "MUI2.nsh"

!ifndef APP_NAME
!define APP_NAME "Syncthing"
!endif
!ifndef APP_VERSION
!define APP_VERSION "dev"
!endif
!ifndef OUTPUT_FILE
!define OUTPUT_FILE "syncthing-installer.exe"
!endif
!ifndef SERVICE_NAME
!define SERVICE_NAME "Syncthing"
!endif
!ifndef SERVICE_DISPLAY_NAME
!define SERVICE_DISPLAY_NAME "${APP_NAME}"
!endif
!ifndef SERVICE_DESCRIPTION
!define SERVICE_DESCRIPTION "${APP_NAME} background service"
!endif
!ifndef BINARY_NAME
!define BINARY_NAME "syncthing.exe"
!endif
!ifndef APP_DATA_DIR
!define APP_DATA_DIR "$PROGRAMDATA\Syncthing"
!endif
!ifndef APP_ARGS
!define APP_ARGS ""
!endif
!ifndef SYNC_SUBDIR
!define SYNC_SUBDIR ""
!endif

Name "${APP_NAME}"
OutFile "${OUTPUT_FILE}"
InstallDir "$PROGRAMFILES64\${APP_NAME}"
InstallDirRegKey HKLM "Software\Syncthing\Install" "Path"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "Install"
  SetOutPath "$INSTDIR"
  File "${BINARY_NAME}"
  FileOpen $0 "$INSTDIR\NOTICE.txt" w
  FileWrite $0 "Installed package: ${APP_NAME} ${APP_VERSION}$\r$\n"
  FileClose $0

  CreateDirectory "${APP_DATA_DIR}"
  CreateDirectory "${APP_DATA_DIR}${SYNC_SUBDIR}"

  WriteRegStr HKLM "Software\Syncthing\Install" "Path" "$INSTDIR"
  WriteRegStr HKLM "Software\Syncthing\Install" "DataDir" "${APP_DATA_DIR}"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "DisplayName" "${APP_NAME}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "NoRepair" 1
  WriteUninstaller "$INSTDIR\uninstall.exe"

  nsExec::ExecToLog 'sc.exe stop "${SERVICE_NAME}"'
  nsExec::ExecToLog 'sc.exe delete "${SERVICE_NAME}"'
  nsExec::ExecToLog 'sc.exe create "${SERVICE_NAME}" binPath= "$\"$INSTDIR\${BINARY_NAME}$\" ${APP_ARGS}" start= auto DisplayName= "${SERVICE_DISPLAY_NAME}"'
  nsExec::ExecToLog 'sc.exe description "${SERVICE_NAME}" "${SERVICE_DESCRIPTION}"'
  nsExec::ExecToLog 'sc.exe start "${SERVICE_NAME}"'
SectionEnd

Section "Uninstall"
  nsExec::ExecToLog 'sc.exe stop "${SERVICE_NAME}"'
  nsExec::ExecToLog 'sc.exe delete "${SERVICE_NAME}"'
  Delete "$INSTDIR\${BINARY_NAME}"
  Delete "$INSTDIR\NOTICE.txt"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"
  DeleteRegKey HKLM "Software\Syncthing\Install"
SectionEnd
