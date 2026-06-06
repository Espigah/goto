; goto - Inno Setup Configuration
; Generates a Windows installer (goto_installer_windows.exe) for end users
; Requirements: Inno Setup 6+ (free download from jrsoftware.org)
; 
; This file is called by GitHub Actions (.github/workflows/windows.yml)
; You can also compile locally after building goto.exe

#define AppName "goto"
#define AppVersion GetEnv("GOTO_VERSION")
#if AppVersion == ""
  #define AppVersion "0.3.16"
#endif
#define AppPublisher "Espigah"
#define AppURL "https://github.com/Espigah/goto"
#define AppExeName "goto.exe"

[Setup]
AppId={{8F9A2C5B-4D6E-4A8F-9B3C-1E7D8A9F2B4C}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}
AppUpdatesURL={#AppURL}
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
AllowNoIcons=yes
LicenseFile=..\..\LICENSE
OutputDir=Output
OutputBaseFilename=goto_installer_windows
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\{#AppExeName}

; Multilanguage support
ShowLanguageDialog=auto

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "addtopath"; Description: "Adicionar ao PATH do sistema"; GroupDescription: "Configuração:"; Flags: checkedonce
Name: "autostart"; Description: "Iniciar automaticamente com o Windows (pausado)"; GroupDescription: "Configuração:"; Flags: unchecked

[Files]
Source: "..\..\goto.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Registry]
; Add to PATH (HKCU for non-admin install)
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NeedsAddPath('{app}'); Tasks: addtopath

; Autostart entry
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "{#AppName}"; ValueData: """{app}\{#AppExeName}"" --paused"; Tasks: autostart; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(AppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

[CustomMessages]
english.LaunchProgram=Launch %1 after installation
brazilianportuguese.LaunchProgram=Executar %1 após a instalação
