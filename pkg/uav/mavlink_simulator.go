package uav

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// UAVState 无人机状态
type UAVState struct {
	// 基本信息
	UAVID      string    `json:"uav_id"`
	NodeName   string    `json:"node_name"`
	SystemTime time.Time `json:"system_time"`

	// GPS信息
	GPS GPSData `json:"gps"`

	// 姿态信息
	Attitude AttitudeData `json:"attitude"`

	// 飞行状态
	Flight FlightData `json:"flight"`

	// 电池信息
	Battery BatteryData `json:"battery"`

	// 任务状态
	Mission MissionData `json:"mission"`

	// 健康状态
	Health HealthData `json:"health"`

	mu sync.RWMutex
}

// GPSData GPS数据
type GPSData struct {
	Latitude         float64   `json:"latitude"`           // 纬度 (度)
	Longitude        float64   `json:"longitude"`          // 经度 (度)
	Altitude         float64   `json:"altitude"`           // 海拔高度 (米)
	RelativeAltitude float64   `json:"relative_altitude"`  // 相对起飞点高度 (米)
	HDOP             float64   `json:"hdop"`               // 水平精度因子
	SatelliteCount   int       `json:"satellite_count"`    // 卫星数量
	FixType          int       `json:"fix_type"`           // GPS定位类型 (0=无, 2=2D, 3=3D)
	GroundSpeed      float64   `json:"ground_speed"`       // 地面速度 (m/s)
	CourseOverGround float64   `json:"course_over_ground"` // 航向 (度)
	Timestamp        time.Time `json:"timestamp"`
}

// AttitudeData 姿态数据
type AttitudeData struct {
	Roll         float64   `json:"roll"`          // 横滚角 (度)
	Pitch        float64   `json:"pitch"`         // 俯仰角 (度)
	Yaw          float64   `json:"yaw"`           // 偏航角/航向 (度)
	RollRate     float64   `json:"roll_rate"`     // 横滚角速度 (度/秒)
	PitchRate    float64   `json:"pitch_rate"`    // 俯仰角速度 (度/秒)
	YawRate      float64   `json:"yaw_rate"`      // 偏航角速度 (度/秒)
	Timestamp    time.Time `json:"timestamp"`
}

// FlightData 飞行数据
type FlightData struct {
	Mode            string    `json:"mode"`              // 飞行模式 (MANUAL, STABILIZE, LOITER, AUTO, RTL, LAND)
	Armed           bool      `json:"armed"`             // 是否解锁
	Airspeed        float64   `json:"airspeed"`          // 空速 (m/s)
	GroundSpeed     float64   `json:"ground_speed"`      // 地速 (m/s)
	VerticalSpeed   float64   `json:"vertical_speed"`    // 垂直速度 (m/s)
	ThrottlePercent float64   `json:"throttle_percent"`  // 油门百分比
	Timestamp       time.Time `json:"timestamp"`
}

// BatteryData 电池数据
type BatteryData struct {
	Voltage            float64   `json:"voltage"`              // 电压 (V)
	Current            float64   `json:"current"`              // 电流 (A)
	RemainingPercent   float64   `json:"remaining_percent"`    // 剩余电量百分比
	RemainingCapacity  float64   `json:"remaining_capacity"`   // 剩余容量 (mAh)
	TotalCapacity      float64   `json:"total_capacity"`       // 总容量 (mAh)
	Temperature        float64   `json:"temperature"`          // 温度 (°C)
	CellCount          int       `json:"cell_count"`           // 电芯数量
	TimeRemaining      int       `json:"time_remaining"`       // 预计剩余时间 (秒)
	Timestamp          time.Time `json:"timestamp"`
}

// MissionData 任务数据
type MissionData struct {
	CurrentWaypoint int       `json:"current_waypoint"`  // 当前航点
	TotalWaypoints  int       `json:"total_waypoints"`   // 总航点数
	MissionState    string    `json:"mission_state"`     // 任务状态 (IDLE, ACTIVE, PAUSED, COMPLETED)
	DistanceToWP    float64   `json:"distance_to_wp"`    // 到下一航点距离 (米)
	ETAToWP         int       `json:"eta_to_wp"`         // 到达航点预计时间 (秒)
	Timestamp       time.Time `json:"timestamp"`
}

// HealthData 健康状态
type HealthData struct {
	SystemStatus     string            `json:"system_status"`      // 系统状态 (OK, WARNING, CRITICAL, ERROR)
	SensorsHealth    map[string]bool   `json:"sensors_health"`     // 传感器健康状态
	ErrorCount       int               `json:"error_count"`        // 错误计数
	WarningCount     int               `json:"warning_count"`      // 警告计数
	Messages         []string          `json:"messages"`           // 状态消息
	LastHeartbeat    time.Time         `json:"last_heartbeat"`     // 最后心跳时间
	Timestamp        time.Time         `json:"timestamp"`
}

// MAVLinkSimulator MAVLink模拟器
type MAVLinkSimulator struct {
	state      *UAVState
	running    bool
	updateRate time.Duration // 更新频率
	stopChan   chan struct{}
	mu         sync.RWMutex
}

// NewMAVLinkSimulator 创建MAVLink模拟器
func NewMAVLinkSimulator(uavID, nodeName string) *MAVLinkSimulator {
	return &MAVLinkSimulator{
		state: &UAVState{
			UAVID:      uavID,
			NodeName:   nodeName,
			SystemTime: time.Now(),
			GPS: GPSData{
				Latitude:       39.9042 + rand.Float64()*0.01, // 北京附近随机位置
				Longitude:      116.4074 + rand.Float64()*0.01,
				Altitude:       50.0,
				FixType:        3,
				SatelliteCount: 12,
				HDOP:           1.0,
			},
			Attitude: AttitudeData{
				Roll:  0,
				Pitch: 0,
				Yaw:   0,
			},
			Flight: FlightData{
				Mode:            "STABILIZE",
				Armed:           false,
				ThrottlePercent: 0,
			},
			Battery: BatteryData{
				Voltage:           22.2,  // 6S电池
				Current:           0.5,   // 待机电流
				RemainingPercent:  100.0,
				RemainingCapacity: 5000.0,
				TotalCapacity:     5000.0,
				Temperature:       25.0,
				CellCount:         6,
			},
			Mission: MissionData{
				CurrentWaypoint: 0,
				TotalWaypoints:  0,
				MissionState:    "IDLE",
			},
			Health: HealthData{
				SystemStatus: "OK",
				SensorsHealth: map[string]bool{
					"gps":          true,
					"compass":      true,
					"accelerometer": true,
					"gyroscope":    true,
					"barometer":    true,
					"battery":      true,
				},
				ErrorCount:   0,
				WarningCount: 0,
				Messages:     []string{},
				LastHeartbeat: time.Now(),
			},
		},
		updateRate: 100 * time.Millisecond, // 10Hz更新频率
		stopChan:   make(chan struct{}),
	}
}

// Start 启动模拟器
func (m *MAVLinkSimulator) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	// 启动模拟循环
	go m.simulationLoop()
}

// Stop 停止模拟器
func (m *MAVLinkSimulator) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)
}

// GetState 获取当前状态（线程安全）
func (m *MAVLinkSimulator) GetState() UAVState {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	// 返回状态副本
	return *m.state
}

// SetFlightMode 设置飞行模式
func (m *MAVLinkSimulator) SetFlightMode(mode string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Flight.Mode = mode
	m.state.Health.Messages = append(m.state.Health.Messages,
		"Flight mode changed to: "+mode)
}

// Arm 解锁
func (m *MAVLinkSimulator) Arm() error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	// 检查是否满足解锁条件
	if m.state.GPS.FixType < 3 {
		return nil // 实际应返回error，这里简化处理
	}

	m.state.Flight.Armed = true
	m.state.Health.Messages = append(m.state.Health.Messages, "Armed")
	return nil
}

// Disarm 上锁
func (m *MAVLinkSimulator) Disarm() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Flight.Armed = false
	m.state.Health.Messages = append(m.state.Health.Messages, "Disarmed")
}

// simulationLoop 模拟循环
func (m *MAVLinkSimulator) simulationLoop() {
	ticker := time.NewTicker(m.updateRate)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.updateState(time.Since(startTime).Seconds())
		}
	}
}

// updateState 更新状态
func (m *MAVLinkSimulator) updateState(elapsedTime float64) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	now := time.Now()

	// 更新GPS（模拟飞行轨迹）
	if m.state.Flight.Armed && m.state.Flight.Mode == "AUTO" {
		// 模拟圆形飞行轨迹
		radius := 0.001 // 约100米半径
		omega := 0.1    // 角速度

		centerLat := 39.9042
		centerLon := 116.4074

		m.state.GPS.Latitude = centerLat + radius*math.Cos(omega*elapsedTime)
		m.state.GPS.Longitude = centerLon + radius*math.Sin(omega*elapsedTime)
		m.state.GPS.RelativeAltitude = 50.0 + 10.0*math.Sin(0.05*elapsedTime)
		m.state.GPS.GroundSpeed = 5.0 + rand.Float64()*0.5
		m.state.GPS.CourseOverGround = math.Mod(omega*elapsedTime*180/math.Pi, 360)
	}
	m.state.GPS.Timestamp = now

	// 更新姿态（模拟飞行姿态变化）
	if m.state.Flight.Armed {
		m.state.Attitude.Roll = 5.0 * math.Sin(0.5*elapsedTime) + rand.Float64()*0.5
		m.state.Attitude.Pitch = 3.0 * math.Cos(0.3*elapsedTime) + rand.Float64()*0.3
		m.state.Attitude.Yaw = math.Mod(m.state.GPS.CourseOverGround, 360)
		m.state.Attitude.RollRate = rand.Float64()*2.0 - 1.0
		m.state.Attitude.PitchRate = rand.Float64()*2.0 - 1.0
		m.state.Attitude.YawRate = rand.Float64()*5.0 - 2.5
	}
	m.state.Attitude.Timestamp = now

	// 更新飞行数据
	if m.state.Flight.Armed {
		m.state.Flight.Airspeed = m.state.GPS.GroundSpeed + rand.Float64()*0.5
		m.state.Flight.GroundSpeed = m.state.GPS.GroundSpeed
		m.state.Flight.VerticalSpeed = math.Cos(0.05*elapsedTime) * 2.0
		m.state.Flight.ThrottlePercent = 50.0 + 20.0*math.Sin(0.1*elapsedTime)
	} else {
		m.state.Flight.ThrottlePercent = 0
		m.state.Flight.VerticalSpeed = 0
	}
	m.state.Flight.Timestamp = now

	// 更新电池（模拟放电）
	if m.state.Flight.Armed {
		// 每秒消耗约0.1%电量
		dischargeRate := 0.1 / (1.0 / m.updateRate.Seconds())
		m.state.Battery.RemainingPercent -= dischargeRate
		if m.state.Battery.RemainingPercent < 0 {
			m.state.Battery.RemainingPercent = 0
		}
		m.state.Battery.RemainingCapacity = m.state.Battery.TotalCapacity * m.state.Battery.RemainingPercent / 100.0
		m.state.Battery.Current = 10.0 + m.state.Flight.ThrottlePercent*0.2
		m.state.Battery.Voltage = 22.2 - (100.0-m.state.Battery.RemainingPercent)*0.04
		m.state.Battery.Temperature = 25.0 + (100.0-m.state.Battery.RemainingPercent)*0.3

		// 计算剩余飞行时间（简化计算）
		if m.state.Battery.Current > 0 {
			m.state.Battery.TimeRemaining = int((m.state.Battery.RemainingCapacity / m.state.Battery.Current) * 3600)
		}
	}
	m.state.Battery.Timestamp = now

	// 更新健康状态
	m.state.Health.LastHeartbeat = now
	m.state.Health.Timestamp = now

	// 检查低电量警告
	if m.state.Battery.RemainingPercent < 20.0 && m.state.Health.SystemStatus == "OK" {
		m.state.Health.SystemStatus = "WARNING"
		m.state.Health.WarningCount++
		m.state.Health.Messages = append(m.state.Health.Messages, "Low battery warning")
	}

	// 检查严重低电量
	if m.state.Battery.RemainingPercent < 10.0 {
		m.state.Health.SystemStatus = "CRITICAL"
		m.state.Health.ErrorCount++
		m.state.Health.Messages = append(m.state.Health.Messages, "Critical battery level - RTL recommended")
	}

	// 限制消息数量
	if len(m.state.Health.Messages) > 10 {
		m.state.Health.Messages = m.state.Health.Messages[len(m.state.Health.Messages)-10:]
	}

	m.state.SystemTime = now
}

// TakeOff 起飞
func (m *MAVLinkSimulator) TakeOff(altitude float64) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	if !m.state.Flight.Armed {
		return
	}

	m.state.Flight.Mode = "AUTO"
	m.state.Mission.MissionState = "ACTIVE"
	m.state.Health.Messages = append(m.state.Health.Messages,
		"Taking off to altitude: " + string(rune(altitude)))
}

// Land 降落
func (m *MAVLinkSimulator) Land() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Flight.Mode = "LAND"
	m.state.Health.Messages = append(m.state.Health.Messages, "Landing initiated")
}

// ReturnToLaunch 返航
func (m *MAVLinkSimulator) ReturnToLaunch() {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Flight.Mode = "RTL"
	m.state.Health.Messages = append(m.state.Health.Messages, "Returning to launch")
}
