#!/bin/bash

echo "=== 多节点日志数据一致性测试 ==="

# 启动集群（后台运行）
echo "启动三节点Raft集群..."
go run . &
CLUSTER_PID=$!

# 等待集群启动
echo "等待集群启动..."
sleep 8

echo ""
echo "1. 检查集群状态:"

# 检查各节点状态
for i in {1..3}; do
    port=$((19000 + i))
    echo "  检查节点 $i (端口 $port):"
    
    status=$(curl -s "http://localhost:$port/cluster/status" 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "    状态: $(echo $status | jq -r '.state // "unknown"')"
        echo "    是否Leader: $(echo $status | jq -r '.is_leader // false')"
        echo "    任期: $(echo $status | jq -r '.term // 0')"
        echo "    提交索引: $(echo $status | jq -r '.commit_index // 0')"
        echo "    最后索引: $(echo $status | jq -r '.last_index // 0')"
    else
        echo "    ❌ 无法连接到节点 $i"
    fi
    echo ""
done

echo "2. 测试数据写入和一致性:"

# 找到Leader节点
leader_port=""
for i in {1..3}; do
    port=$((19000 + i))
    is_leader=$(curl -s "http://localhost:$port/cluster/status" 2>/dev/null | jq -r '.is_leader // false')
    if [ "$is_leader" = "true" ]; then
        leader_port=$port
        echo "  找到Leader节点: 端口 $port"
        break
    fi
done

if [ -z "$leader_port" ]; then
    echo "  ❌ 没有找到Leader节点"
else
    echo "  在Leader节点写入测试数据..."
    
    # 写入测试数据
    keys=("consistency-test-1" "consistency-test-2" "consistency-test-3")
    values=("value1" "value2" "value3")
    
    for i in "${!keys[@]}"; do
        key="${keys[$i]}"
        value="${values[$i]}"
        echo "    设置 $key=$value"
        
        response=$(curl -s -X PUT "http://localhost:$leader_port/kv/$key" \
            -H "Content-Type: application/json" \
            -d "{\"value\": \"$value\"}" 2>/dev/null)
        
        if [ $? -eq 0 ]; then
            echo "      ✅ 设置成功"
        else
            echo "      ❌ 设置失败"
        fi
        sleep 0.5  # 等待复制
    done
    
    echo ""
    echo "  等待数据复制..."
    sleep 3
    
    echo "  验证所有节点数据一致性:"
    
    # 验证数据一致性
    for i in "${!keys[@]}"; do
        key="${keys[$i]}"
        expected_value="${values[$i]}"
        echo "    检查键 '$key' (期望值: $expected_value):"
        
        consistent=true
        for i in {1..3}; do
            port=$((19000 + i))
            
            actual_value=$(curl -s "http://localhost:$port/kv/$key" 2>/dev/null | jq -r '.value // ""')
            if [ $? -eq 0 ] && [ -n "$actual_value" ]; then
                if [ "$actual_value" = "$expected_value" ]; then
                    echo "      节点 $i: ✅ $actual_value"
                else
                    echo "      节点 $i: ❌ $actual_value (期望: $expected_value)"
                    consistent=false
                fi
            else
                echo "      节点 $i: ❌ 无法获取数据"
                consistent=false
            fi
        done
        
        if [ "$consistent" = true ]; then
            echo "      ✅ 键 '$key' 在所有节点上一致"
        else
            echo "      ❌ 键 '$key' 在节点间不一致"
        fi
        echo ""
    done
fi

echo "3. 检查日志复制状态:"

for i in {1..3}; do
    port=$((19000 + i))
    echo "  节点 $i 日志状态:"
    
    logs=$(curl -s "http://localhost:$port/cluster/logs" 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "    首索引: $(echo $logs | jq -r '.first_index // 0')"
        echo "    末索引: $(echo $logs | jq -r '.last_index // 0')"
        echo "    提交索引: $(echo $logs | jq -r '.commit_index // 0')"
    else
        echo "    ❌ 无法获取日志状态"
    fi
    echo ""
done

echo "4. 最终一致性验证:"

# 获取所有节点的最终状态
echo "  检查所有节点的最终状态:"
terms=()
commits=()

for i in {1..3}; do
    port=$((19000 + i))
    
    status=$(curl -s "http://localhost:$port/cluster/status" 2>/dev/null)
    if [ $? -eq 0 ]; then
        term=$(echo $status | jq -r '.term // 0')
        commit=$(echo $status | jq -r '.commit_index // 0')
        
        terms+=($term)
        commits+=($commit)
        
        echo "    节点 $i: 任期=$term, 提交索引=$commit"
    else
        echo "    节点 $i: ❌ 无法获取状态"
    fi
done

# 检查一致性
if [ ${#terms[@]} -gt 1 ]; then
    first_term=${terms[0]}
    first_commit=${commits[0]}
    
    consistent=true
    for i in "${!terms[@]}"; do
        if [ "${terms[$i]}" != "$first_term" ]; then
            echo "  ❌ 节点任期不一致"
            consistent=false
        fi
        if [ "${commits[$i]}" != "$first_commit" ]; then
            echo "  ❌ 节点提交索引不一致"
            consistent=false
        fi
    done
    
    if [ "$consistent" = true ]; then
        echo "  ✅ 所有节点状态一致"
    fi
fi

echo ""
echo "=== 测试完成 ==="

# 停止集群
echo "停止集群..."
kill $CLUSTER_PID 2>/dev/null
wait $CLUSTER_PID 2>/dev/null

echo "测试结束"