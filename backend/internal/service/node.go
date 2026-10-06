package service

import (
	"fmt"

	"lxdapi/internal/db"
	"lxdapi/models"
)

// NodeInput 节点输入
type NodeInput struct {
	Name       string `json:"name"`
	APIBaseURL string `json:"api_base_url"`
	APIHash    string `json:"api_hash"`
	Region     string `json:"region"`
	Remark     string `json:"remark"`
}

// ListNodes 节点列表
func ListNodes() ([]models.Node, error) {
	var nodes []models.Node
	if err := db.DB.Order("id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetNodes 节点列表（兼容旧调用）
func GetNodes() ([]models.Node, error) {
	return ListNodes()
}

// GetNode 获取节点
func GetNode(id uint) (*models.Node, error) {
	var node models.Node
	if err := db.DB.First(&node, id).Error; err != nil {
		return nil, fmt.Errorf("节点不存在")
	}
	return &node, nil
}

// CreateNode 新增节点
func CreateNode(in NodeInput) (*models.Node, error) {
	if in.Name == "" || in.APIBaseURL == "" || in.APIHash == "" {
		return nil, fmt.Errorf("节点名称、API地址、API密钥均不能为空")
	}
	var cnt int64
	db.DB.Model(&models.Node{}).Where("name = ?", in.Name).Count(&cnt)
	if cnt > 0 {
		return nil, fmt.Errorf("节点名称已存在")
	}
	node := models.Node{Name: in.Name, APIBaseURL: in.APIBaseURL, APIHash: in.APIHash, Region: in.Region, Remark: in.Remark}
	if err := db.DB.Create(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// UpdateNode 更新节点
func UpdateNode(id uint, in NodeInput) error {
	var node models.Node
	if err := db.DB.First(&node, id).Error; err != nil {
		return fmt.Errorf("节点不存在")
	}
	return db.DB.Model(&node).Updates(map[string]interface{}{
		"name":         in.Name,
		"api_base_url": in.APIBaseURL,
		"api_hash":     in.APIHash,
		"region":       in.Region,
		"remark":       in.Remark,
	}).Error
}

// DeleteNode 删除节点（存在关联实例时拒绝）
func DeleteNode(id uint) error {
	var cnt int64
	db.DB.Model(&models.UserProduct{}).Where("node_id = ?", id).Count(&cnt)
	if cnt > 0 {
		return fmt.Errorf("该节点下仍有 %d 个用户实例，无法删除", cnt)
	}
	if err := db.DB.Delete(&models.Node{}, id).Error; err != nil {
		return err
	}
	return nil
}

// TestNode 测试节点连通性
func TestNode(node *models.Node) error {
	client := NewNodeClient(node)
	return client.Ping()
}

// TestNodeByID 按 ID 测试节点
func TestNodeByID(id uint) (string, error) {
	node, err := GetNode(id)
	if err != nil {
		return "", err
	}
	if err := TestNode(node); err != nil {
		return "", fmt.Errorf("节点不可达: %v", err)
	}
	return "online", nil
}

// NodeClientDelete 通过节点客户端删除实例
func NodeClientDelete(nodeID uint, name string, isVM bool) error {
	node, err := GetNode(nodeID)
	if err != nil {
		return err
	}
	return NewNodeClient(node).DeleteContainer(name)
}

// GetNodeClientByProduct 根据实例取对应节点客户端；节点为 0 表示本机
func GetNodeClientByProduct(up *models.UserProduct) *NodeClient {
	if up.NodeID == 0 {
		return nil // 本机
	}
	node, err := GetNode(up.NodeID)
	if err != nil {
		return nil
	}
	return NewNodeClient(node)
}
