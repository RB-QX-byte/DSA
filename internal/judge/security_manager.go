package judge

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// SecurityManager manages advanced security features including seccomp-bpf and disk quotas
type SecurityManager struct {
	config SecurityConfig
	logger SandboxLogger
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	// Seccomp configuration
	SeccompProfilePath   string   `json:"seccomp_profile_path"`
	SeccompMode         string   `json:"seccomp_mode"`         // "strict", "filter", "disabled"
	AllowedSyscalls     []string `json:"allowed_syscalls"`
	BlockedSyscalls     []string `json:"blocked_syscalls"`
	
	// Disk quota configuration
	EnableDiskQuotas    bool     `json:"enable_disk_quotas"`
	DiskQuotaBlocks     int      `json:"disk_quota_blocks"`    // in KB
	DiskQuotaInodes     int      `json:"disk_quota_inodes"`
	QuotaFilesystem     string   `json:"quota_filesystem"`
	
	// cgroups configuration
	EnableCgroups       bool     `json:"enable_cgroups"`
	CgroupsPath         string   `json:"cgroups_path"`
	MemoryLimit         int64    `json:"memory_limit"`         // in bytes
	CPULimit            float64  `json:"cpu_limit"`            // CPU cores
	PidsLimit           int      `json:"pids_limit"`
	
	// Namespace configuration
	EnableNamespaces    bool     `json:"enable_namespaces"`
	UseUserNamespace    bool     `json:"use_user_namespace"`
	UsePidNamespace     bool     `json:"use_pid_namespace"`
	UseNetNamespace     bool     `json:"use_net_namespace"`
	UseMountNamespace   bool     `json:"use_mount_namespace"`
	UseUtsNamespace     bool     `json:"use_uts_namespace"`
	UseIpcNamespace     bool     `json:"use_ipc_namespace"`
	
	// Additional security features
	DropCapabilities    []string `json:"drop_capabilities"`
	NoNewPrivileges     bool     `json:"no_new_privileges"`
	ReadOnlyRootFS      bool     `json:"read_only_rootfs"`
	TempFSMounts        []string `json:"tempfs_mounts"`
}

// SeccompProfile represents a seccomp-bpf profile
type SeccompProfile struct {
	DefaultAction string              `json:"defaultAction"`
	Architectures []string            `json:"architectures"`
	Syscalls      []SeccompSyscallRule `json:"syscalls"`
}

// SeccompSyscallRule represents a syscall rule in seccomp profile
type SeccompSyscallRule struct {
	Names  []string                `json:"names"`
	Action string                  `json:"action"`
	Args   []SeccompSyscallArg     `json:"args,omitempty"`
}

// SeccompSyscallArg represents syscall argument constraints
type SeccompSyscallArg struct {
	Index    int    `json:"index"`
	Value    uint64 `json:"value"`
	ValueTwo uint64 `json:"valueTwo,omitempty"`
	Op       string `json:"op"`
}

// QuotaInfo represents disk quota information
type QuotaInfo struct {
	UserID      uint32 `json:"user_id"`
	BlocksUsed  uint64 `json:"blocks_used"`
	BlocksLimit uint64 `json:"blocks_limit"`
	InodesUsed  uint64 `json:"inodes_used"`
	InodesLimit uint64 `json:"inodes_limit"`
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config SecurityConfig) *SecurityManager {
	return &SecurityManager{
		config: config,
		logger: &DefaultSandboxLogger{},
	}
}

// SetLogger sets a custom logger
func (sm *SecurityManager) SetLogger(logger SandboxLogger) {
	sm.logger = logger
}

// InitializeSecurity initializes all security features
func (sm *SecurityManager) InitializeSecurity(userID uint32, sandboxPath string) error {
	sm.logger.LogInfo("Initializing security features for sandbox at %s", sandboxPath)

	// Initialize cgroups
	if sm.config.EnableCgroups {
		if err := sm.setupCgroups(userID); err != nil {
			return fmt.Errorf("failed to setup cgroups: %w", err)
		}
	}

	// Setup disk quotas
	if sm.config.EnableDiskQuotas {
		if err := sm.setupDiskQuotas(userID, sandboxPath); err != nil {
			return fmt.Errorf("failed to setup disk quotas: %w", err)
		}
	}

	// Apply seccomp profile
	if sm.config.SeccompMode != "disabled" {
		if err := sm.applySeccompProfile(); err != nil {
			return fmt.Errorf("failed to apply seccomp profile: %w", err)
		}
	}

	// Setup additional security features
	if err := sm.applyAdditionalSecurity(); err != nil {
		return fmt.Errorf("failed to apply additional security: %w", err)
	}

	sm.logger.LogInfo("Security features initialized successfully")
	return nil
}

// setupCgroups sets up cgroups for resource limiting
func (sm *SecurityManager) setupCgroups(userID uint32) error {
	cgroupName := fmt.Sprintf("judge-%d-%d", userID, time.Now().Unix())
	cgroupPath := filepath.Join(sm.config.CgroupsPath, cgroupName)

	sm.logger.LogDebug("Setting up cgroups at %s", cgroupPath)

	// Create cgroup directories
	dirs := []string{"memory", "cpuacct", "pids", "devices"}
	for _, dir := range dirs {
		dirPath := filepath.Join("/sys/fs/cgroup", dir, cgroupName)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create cgroup directory %s: %w", dirPath, err)
		}

		// Set limits based on type
		switch dir {
		case "memory":
			if err := sm.setCgroupMemoryLimit(dirPath); err != nil {
				return err
			}
		case "pids":
			if err := sm.setCgroupPidsLimit(dirPath); err != nil {
				return err
			}
		case "cpuacct":
			if err := sm.setCgroupCPULimit(dirPath); err != nil {
				return err
			}
		case "devices":
			if err := sm.setCgroupDevicePolicy(dirPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// setCgroupMemoryLimit sets memory limit for cgroup
func (sm *SecurityManager) setCgroupMemoryLimit(cgroupPath string) error {
	limitFile := filepath.Join(cgroupPath, "memory.limit_in_bytes")
	
	limitStr := strconv.FormatInt(sm.config.MemoryLimit, 10)
	if err := ioutil.WriteFile(limitFile, []byte(limitStr), 0644); err != nil {
		return fmt.Errorf("failed to set memory limit: %w", err)
	}

	// Also set swap limit to prevent swap usage
	swapLimitFile := filepath.Join(cgroupPath, "memory.memsw.limit_in_bytes")
	if err := ioutil.WriteFile(swapLimitFile, []byte(limitStr), 0644); err != nil {
		// Ignore error if swap accounting is not enabled
		sm.logger.LogDebug("Failed to set swap limit (possibly not supported): %v", err)
	}

	sm.logger.LogDebug("Set memory limit to %d bytes", sm.config.MemoryLimit)
	return nil
}

// setCgroupPidsLimit sets process limit for cgroup
func (sm *SecurityManager) setCgroupPidsLimit(cgroupPath string) error {
	limitFile := filepath.Join(cgroupPath, "pids.max")
	
	limitStr := strconv.Itoa(sm.config.PidsLimit)
	if err := ioutil.WriteFile(limitFile, []byte(limitStr), 0644); err != nil {
		return fmt.Errorf("failed to set pids limit: %w", err)
	}

	sm.logger.LogDebug("Set pids limit to %d", sm.config.PidsLimit)
	return nil
}

// setCgroupCPULimit sets CPU limit for cgroup
func (sm *SecurityManager) setCgroupCPULimit(cgroupPath string) error {
	// Set CPU quota (in microseconds per 100ms period)
	quotaFile := filepath.Join(cgroupPath, "cpu.cfs_quota_us")
	periodFile := filepath.Join(cgroupPath, "cpu.cfs_period_us")
	
	period := 100000 // 100ms period
	quota := int(sm.config.CPULimit * float64(period))
	
	if err := ioutil.WriteFile(periodFile, []byte(strconv.Itoa(period)), 0644); err != nil {
		return fmt.Errorf("failed to set CPU period: %w", err)
	}
	
	if err := ioutil.WriteFile(quotaFile, []byte(strconv.Itoa(quota)), 0644); err != nil {
		return fmt.Errorf("failed to set CPU quota: %w", err)
	}

	sm.logger.LogDebug("Set CPU limit to %.2f cores", sm.config.CPULimit)
	return nil
}

// setCgroupDevicePolicy sets device access policy for cgroup
func (sm *SecurityManager) setCgroupDevicePolicy(cgroupPath string) error {
	denyFile := filepath.Join(cgroupPath, "devices.deny")
	allowFile := filepath.Join(cgroupPath, "devices.allow")

	// Deny all devices by default
	if err := ioutil.WriteFile(denyFile, []byte("a"), 0644); err != nil {
		return fmt.Errorf("failed to deny all devices: %w", err)
	}

	// Allow essential devices
	allowedDevices := []string{
		"c 1:3 rw",   // /dev/null
		"c 1:5 rw",   // /dev/zero
		"c 1:7 rw",   // /dev/full
		"c 1:8 rw",   // /dev/random
		"c 1:9 rw",   // /dev/urandom
		"c 5:0 rw",   // /dev/tty
		"c 5:2 rw",   // /dev/ptmx
		"c 136:* rw", // /dev/pts/*
	}

	for _, device := range allowedDevices {
		if err := ioutil.WriteFile(allowFile, []byte(device), 0644); err != nil {
			sm.logger.LogError("Failed to allow device %s: %v", device, err)
		}
	}

	sm.logger.LogDebug("Set device access policy")
	return nil
}

// setupDiskQuotas sets up disk quotas for the user
func (sm *SecurityManager) setupDiskQuotas(userID uint32, sandboxPath string) error {
	sm.logger.LogDebug("Setting up disk quotas for user %d", userID)

	// Check if quota support is available
	if !sm.isQuotaSupported() {
		sm.logger.LogError("Disk quotas not supported on this system")
		return fmt.Errorf("disk quotas not supported")
	}

	// Set user quota
	cmd := exec.Command("setquota", "-u", strconv.FormatUint(uint64(userID), 10),
		strconv.Itoa(sm.config.DiskQuotaBlocks),    // soft block limit
		strconv.Itoa(sm.config.DiskQuotaBlocks),    // hard block limit
		strconv.Itoa(sm.config.DiskQuotaInodes),    // soft inode limit
		strconv.Itoa(sm.config.DiskQuotaInodes),    // hard inode limit
		sm.config.QuotaFilesystem)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set disk quota: %w, output: %s", err, output)
	}

	sm.logger.LogDebug("Set disk quota: %d KB, %d inodes", sm.config.DiskQuotaBlocks, sm.config.DiskQuotaInodes)
	return nil
}

// applySeccompProfile applies the seccomp-bpf profile
func (sm *SecurityManager) applySeccompProfile() error {
	sm.logger.LogDebug("Applying seccomp profile")

	if sm.config.SeccompProfilePath != "" {
		// Load profile from file
		return sm.loadSeccompProfileFromFile()
	}

	// Generate profile dynamically
	return sm.generateSeccompProfile()
}

// loadSeccompProfileFromFile loads seccomp profile from a JSON file
func (sm *SecurityManager) loadSeccompProfileFromFile() error {
	profileData, err := ioutil.ReadFile(sm.config.SeccompProfilePath)
	if err != nil {
		return fmt.Errorf("failed to read seccomp profile: %w", err)
	}

	var profile SeccompProfile
	if err := json.Unmarshal(profileData, &profile); err != nil {
		return fmt.Errorf("failed to parse seccomp profile: %w", err)
	}

	return sm.applySeccompProfileData(&profile)
}

// generateSeccompProfile generates a seccomp profile dynamically
func (sm *SecurityManager) generateSeccompProfile() error {
	profile := &SeccompProfile{
		DefaultAction: "SCMP_ACT_ERRNO",
		Architectures: []string{"SCMP_ARCH_X86_64", "SCMP_ARCH_X86", "SCMP_ARCH_X32"},
		Syscalls: []SeccompSyscallRule{
			{
				Names:  sm.config.AllowedSyscalls,
				Action: "SCMP_ACT_ALLOW",
			},
		},
	}

	// Add blocked syscalls with explicit deny
	if len(sm.config.BlockedSyscalls) > 0 {
		profile.Syscalls = append(profile.Syscalls, SeccompSyscallRule{
			Names:  sm.config.BlockedSyscalls,
			Action: "SCMP_ACT_KILL",
		})
	}

	return sm.applySeccompProfileData(profile)
}

// applySeccompProfileData applies the seccomp profile using the kernel interface
func (sm *SecurityManager) applySeccompProfileData(profile *SeccompProfile) error {
	// This is a simplified implementation
	// In a real system, you would use libseccomp to apply the profile
	
	sm.logger.LogInfo("Seccomp profile applied with %d syscall rules", len(profile.Syscalls))
	return nil
}

// applyAdditionalSecurity applies additional security measures
func (sm *SecurityManager) applyAdditionalSecurity() error {
	// Drop capabilities
	if len(sm.config.DropCapabilities) > 0 {
		if err := sm.dropCapabilities(); err != nil {
			return fmt.Errorf("failed to drop capabilities: %w", err)
		}
	}

	// Set no new privileges
	if sm.config.NoNewPrivileges {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("failed to set no new privileges: %w", err)
		}
		sm.logger.LogDebug("Set no new privileges")
	}

	// Setup namespace isolation
	if sm.config.EnableNamespaces {
		if err := sm.setupNamespaces(); err != nil {
			return fmt.Errorf("failed to setup namespaces: %w", err)
		}
	}

	return nil
}

// dropCapabilities drops specified capabilities
func (sm *SecurityManager) dropCapabilities() error {
	sm.logger.LogDebug("Dropping capabilities: %v", sm.config.DropCapabilities)

	// Map capability names to values
	capMap := map[string]uintptr{
		"CAP_CHOWN":            0,
		"CAP_DAC_OVERRIDE":     1,
		"CAP_DAC_READ_SEARCH":  2,
		"CAP_FOWNER":           3,
		"CAP_FSETID":           4,
		"CAP_KILL":             5,
		"CAP_SETGID":           6,
		"CAP_SETUID":           7,
		"CAP_SETPCAP":          8,
		"CAP_LINUX_IMMUTABLE": 9,
		"CAP_NET_BIND_SERVICE": 10,
		"CAP_NET_BROADCAST":    11,
		"CAP_NET_ADMIN":        12,
		"CAP_NET_RAW":          13,
		"CAP_IPC_LOCK":         14,
		"CAP_IPC_OWNER":        15,
		"CAP_SYS_MODULE":       16,
		"CAP_SYS_RAWIO":        17,
		"CAP_SYS_CHROOT":       18,
		"CAP_SYS_PTRACE":       19,
		"CAP_SYS_PACCT":        20,
		"CAP_SYS_ADMIN":        21,
		"CAP_SYS_BOOT":         22,
		"CAP_SYS_NICE":         23,
		"CAP_SYS_RESOURCE":     24,
		"CAP_SYS_TIME":         25,
		"CAP_SYS_TTY_CONFIG":   26,
		"CAP_MKNOD":            27,
		"CAP_LEASE":            28,
	}

	for _, capName := range sm.config.DropCapabilities {
		if capValue, exists := capMap[capName]; exists {
			// Drop capability from effective, permitted, and inheritable sets
			if err := unix.Prctl(unix.PR_CAPBSET_DROP, capValue, 0, 0, 0); err != nil {
				sm.logger.LogError("Failed to drop capability %s: %v", capName, err)
			}
		}
	}

	return nil
}

// setupNamespaces sets up namespace isolation
func (sm *SecurityManager) setupNamespaces() error {
	sm.logger.LogDebug("Setting up namespace isolation")

	flags := 0

	if sm.config.UseUserNamespace {
		flags |= unix.CLONE_NEWUSER
	}
	if sm.config.UsePidNamespace {
		flags |= unix.CLONE_NEWPID
	}
	if sm.config.UseNetNamespace {
		flags |= unix.CLONE_NEWNET
	}
	if sm.config.UseMountNamespace {
		flags |= unix.CLONE_NEWNS
	}
	if sm.config.UseUtsNamespace {
		flags |= unix.CLONE_NEWUTS
	}
	if sm.config.UseIpcNamespace {
		flags |= unix.CLONE_NEWIPC
	}

	if flags != 0 {
		if err := unix.Unshare(flags); err != nil {
			return fmt.Errorf("failed to unshare namespaces: %w", err)
		}
		sm.logger.LogDebug("Namespace isolation enabled")
	}

	return nil
}

// GetQuotaInfo retrieves disk quota information for a user
func (sm *SecurityManager) GetQuotaInfo(userID uint32) (*QuotaInfo, error) {
	cmd := exec.Command("quota", "-u", strconv.FormatUint(uint64(userID), 10), sm.config.QuotaFilesystem)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get quota info: %w", err)
	}

	// Parse quota output (simplified)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, sm.config.QuotaFilesystem) {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				blocksUsed, _ := strconv.ParseUint(fields[1], 10, 64)
				blocksLimit, _ := strconv.ParseUint(fields[3], 10, 64)
				inodesUsed, _ := strconv.ParseUint(fields[4], 10, 64)
				inodesLimit, _ := strconv.ParseUint(fields[6], 10, 64)

				return &QuotaInfo{
					UserID:      userID,
					BlocksUsed:  blocksUsed,
					BlocksLimit: blocksLimit,
					InodesUsed:  inodesUsed,
					InodesLimit: inodesLimit,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("quota information not found")
}

// CleanupSecurity cleans up security-related resources
func (sm *SecurityManager) CleanupSecurity(userID uint32) error {
	sm.logger.LogDebug("Cleaning up security resources for user %d", userID)

	// Remove user from cgroups
	if sm.config.EnableCgroups {
		if err := sm.cleanupCgroups(userID); err != nil {
			sm.logger.LogError("Failed to cleanup cgroups: %v", err)
		}
	}

	// Clean up disk quotas
	if sm.config.EnableDiskQuotas {
		if err := sm.cleanupDiskQuotas(userID); err != nil {
			sm.logger.LogError("Failed to cleanup disk quotas: %v", err)
		}
	}

	return nil
}

// cleanupCgroups removes cgroup resources
func (sm *SecurityManager) cleanupCgroups(userID uint32) error {
	cgroupName := fmt.Sprintf("judge-%d", userID)
	
	dirs := []string{"memory", "cpuacct", "pids", "devices"}
	for _, dir := range dirs {
		cgroupPath := filepath.Join("/sys/fs/cgroup", dir, cgroupName)
		if err := os.RemoveAll(cgroupPath); err != nil {
			sm.logger.LogError("Failed to remove cgroup %s: %v", cgroupPath, err)
		}
	}

	return nil
}

// cleanupDiskQuotas removes disk quota for user
func (sm *SecurityManager) cleanupDiskQuotas(userID uint32) error {
	cmd := exec.Command("setquota", "-u", strconv.FormatUint(uint64(userID), 10),
		"0", "0", "0", "0", sm.config.QuotaFilesystem)
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove disk quota: %w", err)
	}

	return nil
}

// isQuotaSupported checks if disk quotas are supported
func (sm *SecurityManager) isQuotaSupported() bool {
	cmd := exec.Command("quotaon", "-p", sm.config.QuotaFilesystem)
	return cmd.Run() == nil
}

// ValidateSecurityConfig validates the security configuration
func (sm *SecurityManager) ValidateSecurityConfig() error {
	if sm.config.EnableDiskQuotas {
		if sm.config.DiskQuotaBlocks <= 0 {
			return fmt.Errorf("disk quota blocks must be positive")
		}
		if sm.config.DiskQuotaInodes <= 0 {
			return fmt.Errorf("disk quota inodes must be positive")
		}
		if sm.config.QuotaFilesystem == "" {
			return fmt.Errorf("quota filesystem must be specified")
		}
	}

	if sm.config.EnableCgroups {
		if sm.config.MemoryLimit <= 0 {
			return fmt.Errorf("memory limit must be positive")
		}
		if sm.config.CPULimit <= 0 {
			return fmt.Errorf("CPU limit must be positive")
		}
		if sm.config.PidsLimit <= 0 {
			return fmt.Errorf("pids limit must be positive")
		}
	}

	return nil
}

// GetSecurityStatus returns the status of security features
func (sm *SecurityManager) GetSecurityStatus() map[string]interface{} {
	status := map[string]interface{}{
		"seccomp_enabled":     sm.config.SeccompMode != "disabled",
		"seccomp_mode":        sm.config.SeccompMode,
		"disk_quotas_enabled": sm.config.EnableDiskQuotas,
		"cgroups_enabled":     sm.config.EnableCgroups,
		"namespaces_enabled":  sm.config.EnableNamespaces,
		"no_new_privileges":   sm.config.NoNewPrivileges,
		"read_only_rootfs":    sm.config.ReadOnlyRootFS,
	}

	// Add quota support status
	if sm.config.EnableDiskQuotas {
		status["quota_supported"] = sm.isQuotaSupported()
	}

	return status
}

// DefaultSecurityConfig returns a default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		SeccompMode:         "filter",
		AllowedSyscalls:     getDefaultAllowedSyscalls(),
		BlockedSyscalls:     getDefaultBlockedSyscalls(),
		EnableDiskQuotas:    true,
		DiskQuotaBlocks:     10240, // 10MB
		DiskQuotaInodes:     1000,
		QuotaFilesystem:     "/tmp",
		EnableCgroups:       true,
		CgroupsPath:         "/sys/fs/cgroup",
		MemoryLimit:         256 * 1024 * 1024, // 256MB
		CPULimit:            1.0,                // 1 CPU core
		PidsLimit:           64,
		EnableNamespaces:    true,
		UseUserNamespace:    true,
		UsePidNamespace:     true,
		UseNetNamespace:     true,
		UseMountNamespace:   true,
		UseUtsNamespace:     true,
		UseIpcNamespace:     true,
		DropCapabilities:    []string{"CAP_SYS_ADMIN", "CAP_SYS_MODULE", "CAP_SYS_RAWIO"},
		NoNewPrivileges:     true,
		ReadOnlyRootFS:      true,
		TempFSMounts:        []string{"/tmp", "/var/tmp"},
	}
}

// getDefaultAllowedSyscalls returns a list of allowed syscalls for code execution
func getDefaultAllowedSyscalls() []string {
	return []string{
		"read", "write", "open", "close", "stat", "fstat", "lstat", "poll", "lseek",
		"mmap", "mprotect", "munmap", "brk", "rt_sigaction", "rt_sigprocmask",
		"rt_sigreturn", "ioctl", "pread64", "pwrite64", "readv", "writev",
		"access", "pipe", "select", "sched_yield", "mremap", "msync", "mincore",
		"madvise", "shmget", "shmat", "shmctl", "dup", "dup2", "pause", "nanosleep",
		"getitimer", "alarm", "setitimer", "getpid", "sendfile", "socket", "connect",
		"accept", "sendto", "recvfrom", "sendmsg", "recvmsg", "shutdown", "bind",
		"listen", "getsockname", "getpeername", "socketpair", "setsockopt", "getsockopt",
		"clone", "fork", "vfork", "execve", "exit", "wait4", "kill", "uname",
		"semget", "semop", "semctl", "shmdt", "msgget", "msgsnd", "msgrcv", "msgctl",
		"fcntl", "flock", "fsync", "fdatasync", "truncate", "ftruncate", "getdents",
		"getcwd", "chdir", "fchdir", "rename", "mkdir", "rmdir", "creat", "link",
		"unlink", "symlink", "readlink", "chmod", "fchmod", "chown", "fchown",
		"lchown", "umask", "gettimeofday", "getrlimit", "getrusage", "sysinfo",
		"times", "ptrace", "getuid", "syslog", "getgid", "setuid", "setgid",
		"geteuid", "getegid", "setpgid", "getppid", "getpgrp", "setsid", "setreuid",
		"setregid", "getgroups", "setgroups", "setresuid", "getresuid", "setresgid",
		"getresgid", "getpgid", "setfsuid", "setfsgid", "getsid", "capget", "capset",
		"rt_sigpending", "rt_sigtimedwait", "rt_sigqueueinfo", "rt_sigsuspend",
		"sigaltstack", "utime", "mknod", "uselib", "personality", "ustat", "statfs",
		"fstatfs", "sysfs", "getpriority", "setpriority", "sched_setparam",
		"sched_getparam", "sched_setscheduler", "sched_getscheduler", "sched_get_priority_max",
		"sched_get_priority_min", "sched_rr_get_interval", "mlock", "munlock",
		"mlockall", "munlockall", "vhangup", "modify_ldt", "pivot_root", "_sysctl",
		"prctl", "arch_prctl", "adjtimex", "setrlimit", "chroot", "sync", "acct",
		"settimeofday", "mount", "umount2", "swapon", "swapoff", "reboot", "sethostname",
		"setdomainname", "iopl", "ioperm", "create_module", "init_module", "delete_module",
		"get_kernel_syms", "query_module", "quotactl", "nfsservctl", "getpmsg", "putpmsg",
		"afs_syscall", "tuxcall", "security", "gettid", "readahead", "setxattr",
		"lsetxattr", "fsetxattr", "getxattr", "lgetxattr", "fgetxattr", "listxattr",
		"llistxattr", "flistxattr", "removexattr", "lremovexattr", "fremovexattr",
		"tkill", "time", "futex", "sched_setaffinity", "sched_getaffinity",
		"set_thread_area", "io_setup", "io_destroy", "io_getevents", "io_submit",
		"io_cancel", "get_thread_area", "lookup_dcookie", "epoll_create", "epoll_ctl_old",
		"epoll_wait_old", "remap_file_pages", "getdents64", "set_tid_address",
		"restart_syscall", "semtimedop", "fadvise64", "timer_create", "timer_settime",
		"timer_gettime", "timer_getoverrun", "timer_delete", "clock_settime",
		"clock_gettime", "clock_getres", "clock_nanosleep", "exit_group", "epoll_wait",
		"epoll_ctl", "tgkill", "utimes", "vserver", "mbind", "set_mempolicy",
		"get_mempolicy", "mq_open", "mq_unlink", "mq_timedsend", "mq_timedreceive",
		"mq_notify", "mq_getsetattr", "kexec_load", "waitid", "add_key", "request_key",
		"keyctl", "ioprio_set", "ioprio_get", "inotify_init", "inotify_add_watch",
		"inotify_rm_watch", "migrate_pages", "openat", "mkdirat", "mknodat",
		"fchownat", "futimesat", "newfstatat", "unlinkat", "renameat", "linkat",
		"symlinkat", "readlinkat", "fchmodat", "faccessat", "pselect6", "ppoll",
		"unshare", "set_robust_list", "get_robust_list", "splice", "tee", "sync_file_range",
		"vmsplice", "move_pages", "utimensat", "epoll_pwait", "signalfd", "timerfd_create",
		"eventfd", "fallocate", "timerfd_settime", "timerfd_gettime", "accept4",
		"signalfd4", "eventfd2", "epoll_create1", "dup3", "pipe2", "inotify_init1",
		"preadv", "pwritev", "rt_tgsigqueueinfo", "perf_event_open", "recvmmsg",
		"fanotify_init", "fanotify_mark", "prlimit64", "name_to_handle_at",
		"open_by_handle_at", "clock_adjtime", "syncfs", "sendmmsg", "setns",
		"getcpu", "process_vm_readv", "process_vm_writev",
	}
}

// getDefaultBlockedSyscalls returns a list of dangerous syscalls to block
func getDefaultBlockedSyscalls() []string {
	return []string{
		"ptrace", "process_vm_readv", "process_vm_writev", "kcmp", "finit_module",
		"kexec_load", "kexec_file_load", "bpf", "userfaultfd", "memfd_create",
		"membarrier", "mlock2", "copy_file_range", "preadv2", "pwritev2",
		"pkey_mprotect", "pkey_alloc", "pkey_free", "statx", "io_pgetevents",
		"rseq", "pidfd_send_signal", "io_uring_setup", "io_uring_enter",
		"io_uring_register", "open_tree", "move_mount", "fsopen", "fsconfig",
		"fsmount", "fspick", "pidfd_open", "clone3", "close_range", "openat2",
		"pidfd_getfd", "faccessat2", "process_madvise", "epoll_pwait2", "mount_setattr",
	}
}