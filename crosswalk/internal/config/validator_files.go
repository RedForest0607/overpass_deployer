package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validateDirectoryDefaultsConfig는 경로 prefix별 기본 소유권 규칙을 검증하고 정규화한다.
func validateDirectoryDefaultsConfig(cfg *DirectoryDefaultsConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if cfg == nil {
		return
	}

	seenPrefixes := make(map[string]struct{}, len(cfg.Rules))
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		fieldPrefix := fmt.Sprintf("%s.rules[%d]", prefix, i)

		rule.Prefix = filepath.ToSlash(filepath.Clean(strings.TrimSpace(rule.Prefix)))
		rule.Owner = strings.TrimSpace(rule.Owner)
		rule.Group = strings.TrimSpace(rule.Group)

		checkUnresolvedEnv(rule.Prefix, fieldPrefix+".prefix")
		checkUnresolvedEnv(rule.Owner, fieldPrefix+".owner")
		checkUnresolvedEnv(rule.Group, fieldPrefix+".group")

		if rule.Prefix == "." || rule.Prefix == "" {
			*errs = append(*errs, fieldPrefix+".prefix is required")
			continue
		}
		if !strings.HasPrefix(rule.Prefix, "/") {
			*errs = append(*errs, fieldPrefix+".prefix must be an absolute path")
		}
		if _, ok := seenPrefixes[rule.Prefix]; ok {
			*errs = append(*errs, fmt.Sprintf("%s.prefix must be unique: %s", fieldPrefix, rule.Prefix))
		}
		seenPrefixes[rule.Prefix] = struct{}{}

		if rule.Owner == "" {
			*errs = append(*errs, fieldPrefix+".owner is required")
		}
		if rule.Group == "" {
			rule.Group = rule.Owner
		}
		if !isSafeOwnerToken(rule.Owner) {
			*errs = append(*errs, fieldPrefix+".owner contains unsupported characters")
		}
		if !isSafeOwnerToken(rule.Group) {
			*errs = append(*errs, fieldPrefix+".group contains unsupported characters")
		}
	}
}

// validateDirectories는 서버 레벨 디렉터리 목록의 빈 값과 미해결 환경 변수를 검사하고 경로 표기를 정리한다.
func validateDirectories(directories *[]string, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if directories == nil {
		return
	}

	for i, dir := range *directories {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		checkUnresolvedEnv(dir, field)
		if strings.TrimSpace(dir) == "" {
			*errs = append(*errs, field+" must not be empty")
			continue
		}
		(*directories)[i] = filepath.ToSlash(strings.TrimSpace(dir))
	}
}

// validateExtraFiles는 추가 파일 배포 항목의 로컬/원격 경로, chmod, 압축 해제 설정을 검증한다.
func validateExtraFiles(extraFiles *[]ExtraFile, prefix string, errs *[]string, checkUnresolvedEnv func(string, string), prodLike bool) {
	if extraFiles == nil {
		return
	}

	for i := range *extraFiles {
		ef := &(*extraFiles)[i]
		fieldPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		checkUnresolvedEnv(ef.LocalPath, fieldPrefix+".local_path")
		if ef.LocalPath == "" {
			*errs = append(*errs, fieldPrefix+".local_path is required")
		} else {
			if strings.HasPrefix(ef.LocalPath, "~/") {
				ef.LocalPath = expandHome(ef.LocalPath)
			}
			rejectProdSampleFile(ef.LocalPath, fieldPrefix+".local_path", prodLike, errs)
			validateExistingFile(ef.LocalPath, fieldPrefix+".local_path", errs)
		}

		checkUnresolvedEnv(ef.RemotePath, fieldPrefix+".remote_path")
		if ef.RemotePath == "" {
			*errs = append(*errs, fieldPrefix+".remote_path is required")
		}

		if ef.Chmod != "" {
			ef.Chmod = strings.TrimSpace(ef.Chmod)
			if !fileModePattern.MatchString(ef.Chmod) {
				*errs = append(*errs, fieldPrefix+".chmod must be a 3 or 4 digit octal mode")
			}
		}

		validateExtractConfig(ef, fieldPrefix+".extract", errs, checkUnresolvedEnv)
	}
}

// validateExtractConfig는 전송 후 tar 압축 해제 옵션을 검증하고 기본 대상 디렉터리를 채운다.
func validateExtractConfig(ef *ExtraFile, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if !ef.Extract.Enabled {
		return
	}

	if !isSupportedTarArchive(ef.RemotePath) {
		*errs = append(*errs, prefix+".enabled supports only .tar, .tar.gz, or .tgz archives")
	}

	checkUnresolvedEnv(ef.Extract.RemoteDir, prefix+".remote_dir")
	if strings.TrimSpace(ef.Extract.RemoteDir) == "" {
		ef.Extract.RemoteDir = filepath.ToSlash(filepath.Dir(ef.RemotePath))
	} else {
		ef.Extract.RemoteDir = filepath.ToSlash(strings.TrimSpace(ef.Extract.RemoteDir))
	}
	if !strings.HasPrefix(ef.Extract.RemoteDir, "/") {
		*errs = append(*errs, prefix+".remote_dir must be an absolute path")
	}

	if ef.Extract.StripComponents < 0 {
		*errs = append(*errs, prefix+".strip_components must be greater than or equal to zero")
	}

	checkUnresolvedEnv(ef.Extract.StagingPath, prefix+".staging_path")
	ef.Extract.StagingPath = filepath.ToSlash(strings.TrimSpace(ef.Extract.StagingPath))
	if ef.Extract.StagingPath != "" && !strings.HasPrefix(ef.Extract.StagingPath, "/") {
		*errs = append(*errs, prefix+".staging_path must be an absolute path")
	}
	if ef.Extract.Sudo && ef.Extract.StagingPath == "" {
		ef.Extract.StagingPath = filepath.ToSlash(filepath.Join("/tmp", filepath.Base(ef.RemotePath)))
	}

	ef.Extract.Owner = strings.TrimSpace(ef.Extract.Owner)
	ef.Extract.Group = strings.TrimSpace(ef.Extract.Group)
	checkUnresolvedEnv(ef.Extract.Owner, prefix+".owner")
	checkUnresolvedEnv(ef.Extract.Group, prefix+".group")
	if ef.Extract.Group == "" {
		ef.Extract.Group = ef.Extract.Owner
	}
	if ef.Extract.Owner != "" && !isSafeOwnerToken(ef.Extract.Owner) {
		*errs = append(*errs, prefix+".owner contains unsupported characters")
	}
	if ef.Extract.Group != "" && !isSafeOwnerToken(ef.Extract.Group) {
		*errs = append(*errs, prefix+".group contains unsupported characters")
	}
}

// isSupportedTarArchive는 원격 tar 명령으로 처리할 압축 파일 확장자인지 확인한다.
func isSupportedTarArchive(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
}

// validateUtilityConfigs는 유틸리티 이름의 중복과 파일/디렉터리/서비스 선언을 검증한다.
func validateUtilityConfigs(utils []UtilityConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string), prodLikeServer bool) {
	seen := make(map[string]struct{}, len(utils))
	for i := range utils {
		utilityPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		utils[i].Name = strings.TrimSpace(utils[i].Name)
		checkUnresolvedEnv(utils[i].Name, utilityPrefix+".name")
		if utils[i].Name == "" {
			*errs = append(*errs, utilityPrefix+".name must not be empty")
		} else if !isValidBastionAlias(utils[i].Name) {
			*errs = append(*errs, utilityPrefix+".name must contain only letters, numbers, dots, hyphens, or underscores")
		} else if _, exists := seen[utils[i].Name]; exists {
			*errs = append(*errs, fmt.Sprintf("%s.name %q must be unique per server", utilityPrefix, utils[i].Name))
		} else {
			seen[utils[i].Name] = struct{}{}
		}

		validateDirectories(&utils[i].Directories, utilityPrefix+".directories", errs, checkUnresolvedEnv)
		validateExtraFiles(&utils[i].Files, utilityPrefix+".files", errs, checkUnresolvedEnv, prodLikeServer)
		validateServiceConfigs(utils[i].Services, utilityPrefix+".services", errs, checkUnresolvedEnv)
		if len(utils[i].Directories) == 0 && len(utils[i].Files) == 0 && len(utils[i].Services) == 0 {
			*errs = append(*errs, utilityPrefix+" must define directories, files, or services")
		}
	}
}
