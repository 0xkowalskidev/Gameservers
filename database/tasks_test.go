package database

import (
	"testing"
	"time"

	"0xkowalskidev/gameservers/models"
)

func TestDatabaseManager_ScheduledTaskCRUD(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a gameserver first
	gameserver := &models.Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.CreateGameserver(gameserver); err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Create scheduled task
	now := time.Now()
	nextRun := now.Add(time.Hour)
	task := &models.ScheduledTask{
		ID:           "test-task",
		GameserverID: gameserver.ID,
		Name:         "Test Restart",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    now,
		UpdatedAt:    now,
		NextRun:      &nextRun,
	}

	// Create
	if err := db.CreateScheduledTask(task); err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	// Get
	retrieved, err := db.GetScheduledTask(task.ID)
	if err != nil {
		t.Fatalf("Get task failed: %v", err)
	}
	if retrieved.Name != task.Name || retrieved.Type != task.Type {
		t.Errorf("Retrieved task data mismatch")
	}
	if retrieved.NextRun == nil || retrieved.NextRun.Unix() != nextRun.Unix() {
		t.Errorf("NextRun time mismatch")
	}

	// Update
	retrieved.Status = models.TaskStatusDisabled
	retrieved.Name = "Updated Task"
	if err := db.UpdateScheduledTask(retrieved); err != nil {
		t.Fatalf("Update task failed: %v", err)
	}

	updated, _ := db.GetScheduledTask(task.ID)
	if updated.Status != models.TaskStatusDisabled || updated.Name != "Updated Task" {
		t.Errorf("Update task data mismatch")
	}

	// List for gameserver
	tasks, err := db.ListScheduledTasksForGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("List tasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// List active tasks
	activeTasks, err := db.ListActiveScheduledTasks()
	if err != nil {
		t.Fatalf("List active tasks failed: %v", err)
	}
	if len(activeTasks) != 0 { // Should be 0 since we disabled the task
		t.Errorf("Expected 0 active tasks, got %d", len(activeTasks))
	}

	// Re-enable task
	updated.Status = models.TaskStatusActive
	db.UpdateScheduledTask(updated)

	activeTasks, _ = db.ListActiveScheduledTasks()
	if len(activeTasks) != 1 {
		t.Errorf("Expected 1 active task, got %d", len(activeTasks))
	}

	// Delete
	if err := db.DeleteScheduledTask(task.ID); err != nil {
		t.Fatalf("Delete task failed: %v", err)
	}
	if _, err := db.GetScheduledTask(task.ID); err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestDatabaseManager_ScheduledTaskCascadeDelete(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create gameserver and task
	gameserver := &models.Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Status: models.StatusStopped, 
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	db.CreateGameserver(gameserver)

	task := &models.ScheduledTask{
		ID:           "test-task",
		GameserverID: gameserver.ID,
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	db.CreateScheduledTask(task)

	// Delete gameserver - should cascade delete the task
	if err := db.DeleteGameserver(gameserver.ID); err != nil {
		t.Fatalf("Delete gameserver failed: %v", err)
	}

	// Verify task was also deleted (cascade)
	if _, err := db.GetScheduledTask(task.ID); err == nil {
		t.Error("Expected task to be cascade deleted")
	}
}

func TestDatabaseManager_GetNonExistentScheduledTask(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	_, err := db.GetScheduledTask("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent scheduled task")
	}
}

func TestDatabaseManager_UpdateNonExistentScheduledTask(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	task := &models.ScheduledTask{
		ID:           "non-existent",
		GameserverID: "test-gs",
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := db.UpdateScheduledTask(task)
	if err == nil {
		t.Error("Expected error for updating non-existent scheduled task")
	}
}

func TestDatabaseManager_DeleteNonExistentScheduledTask(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	err := db.DeleteScheduledTask("non-existent")
	if err == nil {
		t.Error("Expected error for deleting non-existent scheduled task")
	}
}

func TestDatabaseManager_ListScheduledTasksForNonExistentGameserver(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	tasks, err := db.ListScheduledTasksForGameserver("non-existent")
	if err != nil {
		t.Fatalf("List tasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks for non-existent gameserver, got %d", len(tasks))
	}
}

func TestDatabaseManager_MultipleScheduledTasks(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	// Create gameserver
	gameserver := &models.Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	db.CreateGameserver(gameserver)

	// Create multiple tasks
	tasks := []*models.ScheduledTask{
		{
			ID:           "task-1",
			GameserverID: gameserver.ID,
			Name:         "Daily Restart",
			Type:         models.TaskTypeRestart,
			Status:       models.TaskStatusActive,
			CronSchedule: "0 2 * * *",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "task-2",
			GameserverID: gameserver.ID,
			Name:         "Weekly Backup",
			Type:         models.TaskTypeBackup,
			Status:       models.TaskStatusActive,
			CronSchedule: "0 3 * * 0",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "task-3",
			GameserverID: gameserver.ID,
			Name:         "Disabled Task",
			Type:         models.TaskTypeRestart,
			Status:       models.TaskStatusDisabled,
			CronSchedule: "0 4 * * *",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	// Create all tasks
	for _, task := range tasks {
		if err := db.CreateScheduledTask(task); err != nil {
			t.Fatalf("Failed to create task %s: %v", task.ID, err)
		}
	}

	// List all tasks for gameserver
	allTasks, err := db.ListScheduledTasksForGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("List tasks failed: %v", err)
	}
	if len(allTasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(allTasks))
	}

	// List active tasks only
	activeTasks, err := db.ListActiveScheduledTasks()
	if err != nil {
		t.Fatalf("List active tasks failed: %v", err)
	}
	if len(activeTasks) != 2 {
		t.Errorf("Expected 2 active tasks, got %d", len(activeTasks))
	}

	// Verify task types
	taskTypes := make(map[models.TaskType]int)
	for _, task := range allTasks {
		taskTypes[task.Type]++
	}
	if taskTypes[models.TaskTypeRestart] != 2 {
		t.Errorf("Expected 2 restart tasks, got %d", taskTypes[models.TaskTypeRestart])
	}
	if taskTypes[models.TaskTypeBackup] != 1 {
		t.Errorf("Expected 1 backup task, got %d", taskTypes[models.TaskTypeBackup])
	}
}

func TestDatabaseManager_ScheduledTaskWithNullNextRun(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	// Create gameserver
	gameserver := &models.Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	db.CreateGameserver(gameserver)

	// Create task without NextRun
	task := &models.ScheduledTask{
		ID:           "test-task",
		GameserverID: gameserver.ID,
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		NextRun:      nil, // Explicitly nil
	}

	if err := db.CreateScheduledTask(task); err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	// Retrieve and verify NextRun is nil
	retrieved, err := db.GetScheduledTask(task.ID)
	if err != nil {
		t.Fatalf("Get task failed: %v", err)
	}
	if retrieved.NextRun != nil {
		t.Error("Expected NextRun to be nil")
	}
}

func TestDatabaseManager_ScheduledTaskDifferentGameservers(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	// Create multiple gameservers
	gameserver1 := &models.Gameserver{
		ID: "gs-1", Name: "Server 1", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	gameserver2 := &models.Gameserver{
		ID: "gs-2", Name: "Server 2", GameID: "valheim", 
		PortMappings: []models.PortMapping{{Protocol: "udp", ContainerPort: 2456, HostPort: 0}}, 
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	db.CreateGameserver(gameserver1)
	db.CreateGameserver(gameserver2)

	// Create tasks for each gameserver
	task1 := &models.ScheduledTask{
		ID:           "task-1",
		GameserverID: gameserver1.ID,
		Name:         "Minecraft Restart",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	task2 := &models.ScheduledTask{
		ID:           "task-2",
		GameserverID: gameserver2.ID,
		Name:         "Valheim Backup",
		Type:         models.TaskTypeBackup,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 3 * * *",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	db.CreateScheduledTask(task1)
	db.CreateScheduledTask(task2)

	// List tasks for each gameserver
	gs1Tasks, err := db.ListScheduledTasksForGameserver(gameserver1.ID)
	if err != nil {
		t.Fatalf("List tasks for gs1 failed: %v", err)
	}
	if len(gs1Tasks) != 1 {
		t.Errorf("Expected 1 task for gameserver1, got %d", len(gs1Tasks))
	}
	if gs1Tasks[0].Name != "Minecraft Restart" {
		t.Errorf("Unexpected task name for gameserver1: %s", gs1Tasks[0].Name)
	}

	gs2Tasks, err := db.ListScheduledTasksForGameserver(gameserver2.ID)
	if err != nil {
		t.Fatalf("List tasks for gs2 failed: %v", err)
	}
	if len(gs2Tasks) != 1 {
		t.Errorf("Expected 1 task for gameserver2, got %d", len(gs2Tasks))
	}
	if gs2Tasks[0].Name != "Valheim Backup" {
		t.Errorf("Unexpected task name for gameserver2: %s", gs2Tasks[0].Name)
	}

	// List all active tasks
	allActiveTasks, err := db.ListActiveScheduledTasks()
	if err != nil {
		t.Fatalf("List all active tasks failed: %v", err)
	}
	if len(allActiveTasks) != 2 {
		t.Errorf("Expected 2 active tasks total, got %d", len(allActiveTasks))
	}
}