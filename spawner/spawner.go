package spawner

import (
	"math/rand"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

type Spawner struct {
	EntityType          world.EntityType
	Delay               int
	Movable             bool
	RequiredPlayerRange int
	MaxNearbyEntities   int
	MaxSpawnDelay       int
	MinSpawnDelay       int
	SpawnCount          int
	SpawnRange          int

	pos cube.Pos
}

// BreakInfo ...
func (s Spawner) BreakInfo() block.BreakInfo {
	return newBreakInfo(5, func(t item.Tool) bool { return false }, func(t item.Tool) bool { return t.ToolType() == item.TypePickaxe }, func(t item.Tool, enchantments []item.Enchantment) []item.Stack { return []item.Stack{} })
}

// Activate ...
func (s Spawner) Activate(pos cube.Pos, clickedFace cube.Face, tx *world.Tx, u item.User, ctx *item.UseContext) bool {
	if s.EntityType != nil {
		return false
	}
	held, _ := u.HeldItems()
	egg, ok := held.Item().(SpawnEgg)
	if held.Empty() || !ok {
		return false
	}
	s.EntityType = egg.Kind
	tx.SetBlock(pos, s, nil)
	ctx.SubtractFromCount(1)
	return true
}

// DecodeNBT ...
func (s Spawner) DecodeNBT(data map[string]any) any {
	// 1) Start with vanilla-ish defaults:
	s.Delay = 20
	s.Movable = true
	s.RequiredPlayerRange = 16
	s.MaxNearbyEntities = 6
	s.MaxSpawnDelay = 800
	s.MinSpawnDelay = 200
	s.SpawnCount = 4
	s.SpawnRange = 4

	// 2) Overlay NBT if present:
	if v, ok := nbtInt(data, "Delay"); ok {
		// allow 0 to be kept
		if v >= 0 {
			s.Delay = v
		}
	}
	if v, ok := nbtByte(data, "isMovable"); ok {
		s.Movable = v == 1
	}
	if v, ok := nbtInt(data, "RequiredPlayerRange"); ok && v > 0 {
		s.RequiredPlayerRange = v
	}
	if v, ok := nbtInt(data, "MaxNearbyEntities"); ok && v > 0 {
		s.MaxNearbyEntities = v
	}
	if v, ok := nbtInt(data, "MaxSpawnDelay"); ok && v > 0 {
		s.MaxSpawnDelay = v
	}
	if v, ok := nbtInt(data, "MinSpawnDelay"); ok && v > 0 {
		s.MinSpawnDelay = v
	}
	if v, ok := nbtInt(data, "SpawnCount"); ok && v > 0 {
		s.SpawnCount = v
	}
	if v, ok := nbtInt(data, "SpawnRange"); ok && v > 0 {
		s.SpawnRange = v
	}

	// Entity ID
	if id, ok := data["EntityIdentifier"].(string); ok && id != "" {
		if et, ok2 := entities[id]; ok2 {
			s.EntityType = et
		}
	}

	// 3) Sanity: make sure delays are consistent.
	if s.MaxSpawnDelay < s.MinSpawnDelay {
		s.MaxSpawnDelay = s.MinSpawnDelay
	}

	return s
}

func castOr[T any](v any, or T) T {
	if v == nil {
		return or
	}
	switch v.(type) {
	case T:
		return v.(T)
	default:
		return or
	}
}

func nbtInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		return int(x), true
	case uint64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func nbtByte(m map[string]any, key string) (byte, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case byte:
		return x, true
	case int8:
		return byte(x), true
	case int16:
		return byte(x), true
	case int32:
		return byte(x), true
	case int:
		return byte(x), true
	default:
		return 0, false
	}
}

// EncodeNBT ...
func (s Spawner) EncodeNBT() map[string]any {
	var entityID string
	if s.EntityType != nil {
		entityID = s.EntityType.EncodeEntity()
	}

	return map[string]any{
		"id":               "MobSpawner",
		"EntityIdentifier": entityID,

		"Delay":         int16(s.Delay),
		"MinSpawnDelay": int16(s.MinSpawnDelay),
		"MaxSpawnDelay": int16(s.MaxSpawnDelay),

		"SpawnCount":          int16(s.SpawnCount),
		"SpawnRange":          int16(s.SpawnRange),
		"MaxNearbyEntities":   int16(s.MaxNearbyEntities),
		"RequiredPlayerRange": int16(s.RequiredPlayerRange),

		"DisplayEntityHeight": float32(1),
		"DisplayEntityWidth":  float32(1),
		"isMovable":           boolToByte(s.Movable),

		"x": int32(s.pos.X()),
		"y": int32(s.pos.Y()),
		"z": int32(s.pos.Z()),
	}
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}

	return 0
}

// Tick ...
func (s Spawner) Tick(_ int64, pos cube.Pos, tx *world.Tx) {
	if s.EntityType == nil {
		return
	}

	s.pos = pos
	s.Delay--

	if s.Delay > 0 {
		tx.SetBlock(pos, s, nil)
		return
	}

	p1 := s.pos.Add(cube.Pos{-s.RequiredPlayerRange, -s.RequiredPlayerRange, -s.RequiredPlayerRange})
	p2 := s.pos.Add(cube.Pos{s.RequiredPlayerRange, s.RequiredPlayerRange, s.RequiredPlayerRange})
	box := cube.Box(float64(p1.X()), float64(p1.Y()), float64(p1.Z()), float64(p2.X()), float64(p2.Y()), float64(p2.Z()))

	playerNearby := false
	for e := range tx.EntitiesWithin(box) {
		if e.H().Type() == player.Type {
			playerNearby = true
			break
		}
	}

	if !playerNearby {
		return
	}

	// Count nearby entities of same type
	nearbyCount := 0
	for e := range tx.EntitiesWithin(box) {
		if e.H().Type() == s.EntityType {
			nearbyCount++
			if nearbyCount >= s.MaxNearbyEntities {
				return // too many already
			}
		}
	}

	spawnCount := rand.Intn(s.SpawnCount)
	blockPos := pos.Vec3()
	for i := 0; i < spawnCount; i++ {
		var spawnPos mgl64.Vec3

		if rand.Float64() > 0.5 {
			spawnPos = blockPos.Add(mgl64.Vec3{rand.Float64() * 1.5, 1, rand.Float64() * 1.5})
		} else {
			spawnPos = blockPos.Sub(mgl64.Vec3{-rand.Float64() * -1.5, -1, -rand.Float64() * -1.5})
		}

		newEnt, ok := newEntities[s.EntityType.EncodeEntity()]
		if !ok {
			return
		}

		tx.AddEntity(newEnt(cube.PosFromVec3(spawnPos), tx))
	}

	s.Delay = rand.Intn(s.MaxSpawnDelay-s.MinSpawnDelay) + s.MinSpawnDelay
	tx.SetBlock(pos, s, nil)
}

// EncodeItem ...
func (s Spawner) EncodeItem() (name string, meta int16) {
	return "minecraft:mob_spawner", 0
}

// EncodeBlock ...
func (s Spawner) EncodeBlock() (string, map[string]any) {
	return "minecraft:mob_spawner", nil
}

// Model ...
func (s Spawner) Model() world.BlockModel {
	return model.Solid{}
}
