package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
	"log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	teams "github.com/dasrick/go-teams-notify/v2"
)

var (
	webhookURLTeams = "SEU_WEBHOOK_DO_TEAMS"
	telegramBotToken = "SEU_TOKEN_DO_TELEGRAM"
	chatID           = int64(SEU_ID_DE_CHAT)
	clusterConfigs   = []string{
		"./configs/kind",
		// "./configs/dev",
	}
)

func main() {
	for {
		for _, clusterConfig := range clusterConfigs {
			err := verificarStatusNodes(clusterConfig)
			if err != nil {
				log.Printf("Erro ao verificar o status do cluster %s: %v\n", clusterConfig, err)
			}
		}
		log.Printf("Executtando !!!")
		time.Sleep(1 * time.Minute)
	}
}

func verificarStatusNodes(clusterConfig string) error {
	clusterName := filepath.Base(clusterConfig)
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", clusterConfig)
	if err != nil {
		return fmt.Errorf("falha ao carregar a configuração do cluster %s: %v", clusterConfig, err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("falha ao criar o cliente do Kubernetes para o cluster %s: %v", clusterConfig, err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("falha ao listar os nós do cluster %s: %v", clusterConfig, err)
	}

	for _, node := range nodes.Items {
		nodeName := node.Name
		nodeStatus := getNodeCondition(&node.Status, corev1.NodeReady)
		if nodeStatus == nil || nodeStatus.Status != corev1.ConditionTrue {
			message := fmt.Sprintf("⚠️ ALERTA: O nó %s no cluster %s está fora do estado \"Ready\"!", nodeName, clusterName)
			err := enviarMensagemTelegram(message)
			if err != nil {
				fmt.Printf("Erro ao enviar mensagem via Telegram: %v\n", err)
			}

			err = enviarMensagemTeams(message)
			if err != nil {
				fmt.Printf("Erro ao enviar mensagem via Teams: %v\n", err)
			}
		}
	}

	return nil
}

func getNodeCondition(status *corev1.NodeStatus, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range status.Conditions {
		condition := status.Conditions[i]
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func enviarMensagemTelegram(message string) error {
	bot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		return fmt.Errorf("falha ao criar o cliente do bot do Telegram: %v", err)
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"

	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem via Telegram: %v", err)
	}

	return nil
}

func enviarMensagemTeams(message string) error {

	msTeams := teams.NewMessageCard()
	msTeams.Title = "ALERTA"
	msTeams.Text = message

	payload, err := json.Marshal(msTeams)
	if err != nil {
		return fmt.Errorf("falha ao serializar a mensagem para JSON: %v", err)
	}

	resp, err := http.Post(webhookURLTeams, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("falha ao enviar mensagem via Teams: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("falha ao enviar mensagem via Teams. Status de resposta: %d", resp.StatusCode)
	}

	return nil
}
